/*
Copyright 2026 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package redis 提供 Redis 中间件管理能力（外部直连，支持单机/哨兵/集群）
package redis

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	goredis "github.com/redis/go-redis/v9"
	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	// 连接与操作超时保护
	redisDialTimeout   = 5 * time.Second
	redisOpTimeout     = 10 * time.Second
	redisPoolSize      = 10
	redisIdleTimeout   = 5 * time.Minute
	defaultPageSize    = 100
	maxPageSize        = 100
	maxStringValueSize = 4096                   // 字符串值最大返回长度（rune）
	maxCollectionSize  = 100                    // 集合类 key 最多返回的元素数
	maxWriteKeySize    = 512                    // 写入 key 最大长度（rune）
	maxWriteValueSize  = 65536                  // 写入 string 值最大长度（rune）
	maxTTLSeconds      = int64(365 * 24 * 3600) // TTL 上限 1 年
	maxBatchDeleteKeys = 500                    // 单次批量删除的 key 数上限
	maxCachedClients   = 64                     // client 连接缓存上限，超过时按最近访问时间清理最旧连接
)

type Interface interface {
	Ping(ctx context.Context, datasourceId int64) (*types.RedisPing, error)
	// PingAdhoc 临时探测（不落库、不缓存连接），用于创建数据源前的连通性验证；仅管理员可调用
	PingAdhoc(ctx context.Context, datasourceId int64, cfg *types.RedisSourceConfig) (*types.RedisPing, error)
	// db 为逻辑库编号（0-15）；nil 表示使用数据源配置的默认 DB；cluster 模式强制 0
	Info(ctx context.Context, datasourceId int64, db *int) (*types.RedisInfo, error)
	// ScanKeys cursor 透传分页：前端保存 cursor，后端无状态；count<=0 用默认值，超上限截断
	ScanKeys(ctx context.Context, datasourceId int64, db *int, cursor uint64, match string, count int64) (*types.RedisScanResult, error)
	GetKeyDetail(ctx context.Context, datasourceId int64, db *int, key string) (*types.RedisKeyDetail, error)
	// 写操作：新增 key（仅 string）/删除 key/修改 TTL；仅管理员可调用
	CreateKey(ctx context.Context, datasourceId int64, db *int, req *types.RedisCreateKeyRequest) error
	DeleteKey(ctx context.Context, datasourceId int64, db *int, key string) error
	// DeleteKeys 批量删除 key，返回实际删除数量；仅管理员可调用
	DeleteKeys(ctx context.Context, datasourceId int64, db *int, keys []string) (*types.RedisDeleteKeysResult, error)
	// UpdateKeyValue 修改 string 类型 key 的值（保持原 TTL）；仅管理员可调用
	UpdateKeyValue(ctx context.Context, datasourceId int64, db *int, req *types.RedisUpdateKeyValueRequest) error
	SetKeyTTL(ctx context.Context, datasourceId int64, db *int, key string, ttl int64) error
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory

	mu      sync.Mutex
	clients map[string]*cachedClient
}

// cachedClient 按数据源+DB 缓存 redis 连接，配置变化（resourceVersion）时重建
type cachedClient struct {
	resourceVersion int64
	fingerprint     string
	client          goredis.UniversalClient
	lastAccess      time.Time
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{
		cc:      cfg,
		factory: f,
		clients: make(map[string]*cachedClient),
	}
}

// requireRedisAdmin 限制高危操作（临时探测 / 写 key）仅超管与内置管理员可执行，
// 与菜单 AdminOnly（IsRoot / IsAdmin）对齐，避免普通用户借 API 做 SSRF 或破坏生产数据。
func requireRedisAdmin(ctx context.Context) error {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return err
	}
	if user.Role != model.RoleRoot && user.Role != model.RoleAdmin {
		return apierrors.ErrForbidden
	}
	return nil
}

// resolveDB 解析生效的逻辑库编号：请求值优先，否则用配置默认；cluster 强制 0
func resolveDB(cfg *types.RedisSourceConfig, db *int) (int, error) {
	requested := cfg.DB
	if db != nil {
		requested = *db
	}
	if requested < 0 || requested > 15 {
		return 0, fmt.Errorf("db must be between 0 and 15")
	}
	if cfg.NormalizeMode() == types.RedisModeCluster {
		return 0, nil
	}
	return requested, nil
}

// validateRedisConfig 按部署模式校验连接配置
func validateRedisConfig(cfg *types.RedisSourceConfig) error {
	if cfg == nil {
		return fmt.Errorf("redis config is required")
	}
	// 统一清理地址空白
	for i := range cfg.Addresses {
		cfg.Addresses[i] = strings.TrimSpace(cfg.Addresses[i])
	}
	switch cfg.NormalizeMode() {
	case types.RedisModeSentinel:
		if strings.TrimSpace(cfg.MasterName) == "" {
			return fmt.Errorf("redis sentinel mode requires master_name")
		}
		if len(cfg.Addresses) == 0 {
			return fmt.Errorf("redis sentinel mode requires at least one sentinel address")
		}
	case types.RedisModeCluster:
		if len(cfg.Addresses) == 0 {
			return fmt.Errorf("redis cluster mode requires at least one node address")
		}
		// Redis Cluster 仅支持 db 0
		cfg.DB = 0
	default:
		if strings.TrimSpace(cfg.Address) == "" {
			return fmt.Errorf("redis standalone mode requires address (host:port)")
		}
	}
	return nil
}

// buildRedisClient 按部署模式构造 redis 客户端（调用方负责 Close）
func buildRedisClient(cfg *types.RedisSourceConfig) (goredis.UniversalClient, error) {
	if err := validateRedisConfig(cfg); err != nil {
		return nil, apierrors.NewError(err, http.StatusBadRequest)
	}
	switch cfg.NormalizeMode() {
	case types.RedisModeSentinel:
		return goredis.NewFailoverClient(&goredis.FailoverOptions{
			MasterName:       strings.TrimSpace(cfg.MasterName),
			SentinelAddrs:    cfg.Addresses,
			Password:         cfg.Password,
			SentinelPassword: cfg.SentinelPassword,
			DB:               cfg.DB,
			DialTimeout:      redisDialTimeout,
			ReadTimeout:      redisOpTimeout,
			WriteTimeout:     redisOpTimeout,
			PoolSize:         redisPoolSize,
			ConnMaxIdleTime:  redisIdleTimeout,
		}), nil
	case types.RedisModeCluster:
		return goredis.NewClusterClient(&goredis.ClusterOptions{
			Addrs:           cfg.Addresses,
			Password:        cfg.Password,
			DialTimeout:     redisDialTimeout,
			ReadTimeout:     redisOpTimeout,
			WriteTimeout:    redisOpTimeout,
			PoolSize:        redisPoolSize,
			ConnMaxIdleTime: redisIdleTimeout,
		}), nil
	default:
		return goredis.NewClient(&goredis.Options{
			Addr:            strings.TrimSpace(cfg.Address),
			Password:        cfg.Password,
			DB:              cfg.DB,
			DialTimeout:     redisDialTimeout,
			ReadTimeout:     redisOpTimeout,
			WriteTimeout:    redisOpTimeout,
			PoolSize:        redisPoolSize,
			ConnMaxIdleTime: redisIdleTimeout,
		}), nil
	}
}

// configFingerprint 配置摘要，用于缓存失效判断
func configFingerprint(cfg *types.RedisSourceConfig) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d|%s",
		cfg.NormalizeMode(), cfg.Address, strings.Join(cfg.Addresses, ","),
		cfg.MasterName, cfg.DB, cfg.Password)
}

// clientFor 获取数据源对应的 redis 客户端（带缓存、配置变更失效与归属校验）
func (c *controller) clientFor(ctx context.Context, datasourceId int64, db *int) (goredis.UniversalClient, *types.RedisSourceConfig, error) {
	object, err := c.factory.Datasource().Get(ctx, datasourceId)
	if err != nil {
		klog.Errorf("failed to get datasource(%d): %v", datasourceId, err)
		return nil, nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, nil, apierrors.NewError(fmt.Errorf("datasource not found"), http.StatusNotFound)
	}
	if (object.Type != model.DatasourceTypeRedis && object.Type != model.DatasourceTypeMiddleware) ||
		object.SubType != model.DatasourceSubTypeRedis {
		return nil, nil, apierrors.NewError(fmt.Errorf("datasource(%d) is not a redis datasource", datasourceId), http.StatusBadRequest)
	}
	if !object.External {
		return nil, nil, apierrors.NewError(fmt.Errorf("redis datasource only supports external direct connection"), http.StatusBadRequest)
	}
	// 归属校验：root/owner 放行，其余须命中角色 scope，与 datasource controller 的 Get 完全一致
	if err := controllerutil.CheckResourceAccess(ctx, c.factory, object.UserId, types.ResourceTypeDatasource, datasourceId); err != nil {
		klog.Warningf("datasource(%d) access denied: %v", datasourceId, err)
		return nil, nil, err
	}

	var cfg types.DatasourceConfig
	if err = cfg.Unmarshal(object.Config); err != nil {
		klog.Errorf("failed to unmarshal datasource(%d) config: %v", datasourceId, err)
		return nil, nil, apierrors.ErrServerInternal
	}
	if cfg.Redis == nil {
		return nil, nil, apierrors.NewError(fmt.Errorf("datasource(%d) missing redis config", datasourceId), http.StatusBadRequest)
	}
	effectiveDB, err := resolveDB(cfg.Redis, db)
	if err != nil {
		return nil, nil, apierrors.NewError(err, http.StatusBadRequest)
	}
	cfg.Redis.DB = effectiveDB

	c.mu.Lock()
	defer c.mu.Unlock()

	cacheKey := fmt.Sprintf("%d:%d", datasourceId, effectiveDB)
	fingerprint := configFingerprint(cfg.Redis)
	if cached, ok := c.clients[cacheKey]; ok &&
		cached.resourceVersion == object.ResourceVersion &&
		cached.fingerprint == fingerprint {
		cached.lastAccess = time.Now()
		return cached.client, cfg.Redis, nil
	}

	// 配置变更或首次连接：关闭旧连接并重建
	if cached, ok := c.clients[cacheKey]; ok {
		_ = cached.client.Close()
		delete(c.clients, cacheKey)
	}

	client, err := buildRedisClient(cfg.Redis)
	if err != nil {
		return nil, nil, err
	}
	c.clients[cacheKey] = &cachedClient{
		resourceVersion: object.ResourceVersion,
		fingerprint:     fingerprint,
		client:          client,
		lastAccess:      time.Now(),
	}
	// 连接缓存上限保护：超过 maxCachedClients 时清理最近访问时间最旧的连接，避免无限增长
	if len(c.clients) > maxCachedClients {
		var oldestKey string
		var oldest time.Time
		for k, cc := range c.clients {
			if oldestKey == "" || cc.lastAccess.Before(oldest) {
				oldestKey, oldest = k, cc.lastAccess
			}
		}
		if oldestKey != "" {
			_ = c.clients[oldestKey].client.Close()
			delete(c.clients, oldestKey)
		}
	}
	return client, cfg.Redis, nil
}

// wrapRedisErr 将 redis 操作错误转换为带语义错误码的网关类错误：
// key 不存在→404；认证失败/连接层故障/其余命令失败→502
// 注意：上游认证失败不能返回 401，前端全局拦截器将 401 视为 Pixiu 登录失效并登出跳转
func wrapRedisErr(err error) error {
	msg := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, goredis.Nil):
		return apierrors.NewError(fmt.Errorf("redis key not found"), http.StatusNotFound)
	case strings.Contains(msg, "wrongpass") || strings.Contains(msg, "noauth") || strings.Contains(msg, "authentication"):
		return apierrors.NewError(fmt.Errorf("redis authentication failed: %v", err), http.StatusBadGateway)
	case errors.Is(err, goredis.ErrClosed) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) ||
		strings.Contains(msg, "connection refused") || strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "network is unreachable"):
		return apierrors.NewError(fmt.Errorf("redis connection failed: %v", err), http.StatusBadGateway)
	default:
		return apierrors.NewError(fmt.Errorf("redis request failed: %v", err), http.StatusBadGateway)
	}
}

func opContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, redisOpTimeout)
}

func (c *controller) Ping(ctx context.Context, datasourceId int64) (*types.RedisPing, error) {
	client, cfg, err := c.clientFor(ctx, datasourceId, nil)
	if err != nil {
		return nil, err
	}

	result := &types.RedisPing{Address: cfg.DisplayAddress(), DB: cfg.DB}
	ctx, cancel := opContext(ctx)
	defer cancel()

	start := time.Now()
	if err = client.Ping(ctx).Err(); err != nil {
		result.Message = err.Error()
		return result, nil
	}
	result.Connected = true
	result.LatencyMs = time.Since(start).Milliseconds()

	// 顺带返回版本信息，失败不影响探测结果
	if info, err := client.Info(ctx, "server").Result(); err == nil {
		result.Version = parseInfoField(info, "redis_version")
	}
	return result, nil
}

// PingAdhoc 使用临时连接探测指定配置，探测完成后立即关闭连接。
// 仅管理员可调用：请求体可指定任意地址，属于认证后 SSRF 面，必须收敛权限。
// datasourceId > 0 时读取已保存的数据源，并在密码为空时补齐已保存密码。
func (c *controller) PingAdhoc(ctx context.Context, datasourceId int64, cfg *types.RedisSourceConfig) (*types.RedisPing, error) {
	if err := requireRedisAdmin(ctx); err != nil {
		return nil, err
	}

	if datasourceId > 0 {
		object, err := c.factory.Datasource().Get(ctx, datasourceId)
		if err != nil || object == nil {
			if err != nil {
				return nil, apierrors.ErrServerInternal
			}
			return nil, apierrors.NewError(fmt.Errorf("datasource not found"), http.StatusNotFound)
		}
		if err = controllerutil.CheckResourceAccess(ctx, c.factory, object.UserId, types.ResourceTypeDatasource, datasourceId); err != nil {
			return nil, err
		}
		var saved types.DatasourceConfig
		if err = saved.Unmarshal(object.Config); err != nil || saved.Redis == nil {
			return nil, apierrors.NewError(fmt.Errorf("invalid Redis datasource config"), http.StatusBadRequest)
		}
		if cfg == nil {
			cfg = &types.RedisSourceConfig{}
		}
		if cfg.Password == "" {
			cfg.Password = saved.Redis.Password
		}
		if cfg.SentinelPassword == "" {
			cfg.SentinelPassword = saved.Redis.SentinelPassword
		}
	}

	if cfg == nil {
		return nil, apierrors.NewError(fmt.Errorf("redis config is required"), http.StatusBadRequest)
	}

	result := &types.RedisPing{Address: cfg.DisplayAddress(), DB: cfg.DB}
	client, err := buildRedisClient(cfg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := opContext(ctx)
	defer cancel()

	start := time.Now()
	if err := client.Ping(ctx).Err(); err != nil {
		result.Message = err.Error()
		return result, nil
	}
	result.Connected = true
	result.LatencyMs = time.Since(start).Milliseconds()

	if info, err := client.Info(ctx, "server").Result(); err == nil {
		result.Version = parseInfoField(info, "redis_version")
	}
	return result, nil
}

func (c *controller) Info(ctx context.Context, datasourceId int64, db *int) (*types.RedisInfo, error) {
	client, _, err := c.clientFor(ctx, datasourceId, db)
	if err != nil {
		return nil, err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	// INFO + DBSIZE 共用一次 pipeline，省一次 RTT
	pipe := client.Pipeline()
	infoCmd := pipe.Info(ctx)
	dbSizeCmd := pipe.DBSize(ctx)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, wrapRedisErr(err)
	}
	raw, err := infoCmd.Result()
	if err != nil {
		return nil, wrapRedisErr(err)
	}
	dbSize, err := dbSizeCmd.Result()
	if err != nil {
		return nil, wrapRedisErr(err)
	}

	info := &types.RedisInfo{
		RedisVersion:     parseInfoField(raw, "redis_version"),
		RedisMode:        parseInfoField(raw, "redis_mode"),
		OS:               parseInfoField(raw, "os"),
		UsedMemoryHuman:  parseInfoField(raw, "used_memory_human"),
		UptimeInSeconds:  parseInfoInt(raw, "uptime_in_seconds"),
		ConnectedClients: parseInfoInt(raw, "connected_clients"),
		KeyspaceHits:     parseInfoInt(raw, "keyspace_hits"),
		KeyspaceMisses:   parseInfoInt(raw, "keyspace_misses"),
		TotalKeys:        dbSize,
		Raw:              raw,

		// 内存详情
		UsedMemory:       parseInfoInt(raw, "used_memory"),
		UsedMemoryRss:    parseInfoInt(raw, "used_memory_rss"),
		UsedMemoryPeak:   parseInfoInt(raw, "used_memory_peak"),
		UsedMemoryLua:    parseInfoInt(raw, "used_memory_lua"),
		MaxMemory:        parseInfoInt(raw, "maxmemory"),
		MaxMemoryHuman:   parseInfoField(raw, "maxmemory_human"),
		MaxMemoryPolicy:  parseInfoField(raw, "maxmemory_policy"),
		MemFragmentation: parseInfoFloat(raw, "mem_fragmentation_ratio"),

		// 命令统计
		TotalCommands:    parseInfoInt(raw, "total_commands_processed"),
		InstantaneousOps: parseInfoInt(raw, "instantaneous_ops_per_sec"),

		// 网络
		NetInputBytes:  parseInfoInt(raw, "net_input_bytes_total"),
		NetOutputBytes: parseInfoInt(raw, "net_output_bytes_total"),

		// 键空间统计
		EvictedKeys: parseInfoInt(raw, "evicted_keys"),
		ExpiredKeys: parseInfoInt(raw, "expired_keys"),

		// 连接
		BlockedClients: parseInfoInt(raw, "blocked_clients"),
		RejectedConns:  parseInfoInt(raw, "rejected_connections"),

		// 持久化
		RdbLastSaveStatus: parseInfoField(raw, "rdb_last_bgsave_status"),
		RdbLastSaveTime:   parseInfoInt(raw, "rdb_last_save_time"),
		AofEnabled:        parseInfoInt(raw, "aof_enabled"),

		// 复制
		Role:            parseInfoField(raw, "role"),
		ConnectedSlaves: parseInfoInt(raw, "connected_slaves"),

		// Keyspace 各 DB 分布
		KeyspaceDBs: parseKeyspaceDBs(raw),
	}
	return info, nil
}

// normalizeScanParams 归一化 SCAN 参数：match 空则用 "*"，count 限制在 (0, maxPageSize] 区间
func normalizeScanParams(match string, count int64) (string, int64) {
	if strings.TrimSpace(match) == "" {
		match = "*"
	}
	if count <= 0 {
		count = defaultPageSize
	}
	if count > maxPageSize {
		count = maxPageSize
	}
	return match, count
}

// ScanKeys cursor 透传分页浏览：前端持有 cursor，后端无状态
func (c *controller) ScanKeys(ctx context.Context, datasourceId int64, db *int, cursor uint64, match string, count int64) (*types.RedisScanResult, error) {
	client, _, err := c.clientFor(ctx, datasourceId, db)
	if err != nil {
		return nil, err
	}
	match, count = normalizeScanParams(match, count)

	ctx, cancel := opContext(ctx)
	defer cancel()

	batch, next, err := client.Scan(ctx, cursor, match, count).Result()
	if err != nil {
		return nil, wrapRedisErr(err)
	}
	// SCAN 可能返回空 batch 但游标未结束（COUNT 仅为提示，match 过滤严时常见），自动续扫避免前端空页
	for len(batch) == 0 && next != 0 {
		batch, next, err = client.Scan(ctx, next, match, count).Result()
		if err != nil {
			return nil, wrapRedisErr(err)
		}
	}

	// pipeline 批量读取 TYPE/TTL，避免逐 key 串行往返
	items := make([]types.RedisKeyItem, 0, len(batch))
	if len(batch) > 0 {
		pipe := client.Pipeline()
		typeCmds := make([]*goredis.StatusCmd, len(batch))
		ttlCmds := make([]*goredis.DurationCmd, len(batch))
		for i, key := range batch {
			typeCmds[i] = pipe.Type(ctx, key)
			ttlCmds[i] = pipe.TTL(ctx, key)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return nil, wrapRedisErr(err)
		}
		for i, key := range batch {
			item := types.RedisKeyItem{Key: key, Type: "unknown", TTL: -1}
			if t, err := typeCmds[i].Result(); err == nil {
				item.Type = t
			}
			if ttl, err := ttlCmds[i].Result(); err == nil {
				item.TTL = int64(ttl.Seconds())
				if ttl < 0 {
					item.TTL = int64(ttl) // -1 永不过期 / -2 不存在
				}
			}
			items = append(items, item)
		}
	}

	return &types.RedisScanResult{Cursor: next, Keys: items}, nil
}

func (c *controller) GetKeyDetail(ctx context.Context, datasourceId int64, db *int, key string) (*types.RedisKeyDetail, error) {
	if strings.TrimSpace(key) == "" {
		return nil, apierrors.NewError(fmt.Errorf("key is required"), http.StatusBadRequest)
	}
	client, _, err := c.clientFor(ctx, datasourceId, db)
	if err != nil {
		return nil, err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	keyType, err := client.Type(ctx, key).Result()
	if err != nil {
		return nil, wrapRedisErr(err)
	}
	if keyType == "none" {
		return nil, apierrors.NewError(fmt.Errorf("key not found"), http.StatusNotFound)
	}

	detail := &types.RedisKeyDetail{Key: key, Type: keyType, TTL: -1}
	if ttl, err := client.TTL(ctx, key).Result(); err == nil {
		detail.TTL = int64(ttl.Seconds())
		if ttl < 0 {
			detail.TTL = int64(ttl)
		}
	}
	if encoding, err := client.ObjectEncoding(ctx, key).Result(); err == nil {
		detail.Encoding = encoding
	}
	if size, err := client.MemoryUsage(ctx, key).Result(); err == nil {
		detail.SizeBytes = size
	}

	detail.Value, detail.Truncated = readKeyValue(ctx, client, key, keyType)
	return detail, nil
}

// CreateKey 新增 string 类型 key；key 已存在时拒绝（防误覆盖）
func (c *controller) CreateKey(ctx context.Context, datasourceId int64, db *int, req *types.RedisCreateKeyRequest) error {
	if err := requireRedisAdmin(ctx); err != nil {
		return err
	}
	if req == nil || strings.TrimSpace(req.Key) == "" {
		return apierrors.NewError(fmt.Errorf("key is required"), http.StatusBadRequest)
	}
	key := strings.TrimSpace(req.Key)
	if runes := []rune(key); len(runes) > maxWriteKeySize {
		return apierrors.NewError(fmt.Errorf("key length exceeds limit %d", maxWriteKeySize), http.StatusBadRequest)
	}
	if runes := []rune(req.Value); int64(len(runes)) > maxWriteValueSize {
		return apierrors.NewError(fmt.Errorf("value length exceeds limit %d", maxWriteValueSize), http.StatusBadRequest)
	}
	if req.TTL > maxTTLSeconds {
		return apierrors.NewError(fmt.Errorf("ttl exceeds limit %d seconds", maxTTLSeconds), http.StatusBadRequest)
	}
	client, _, err := c.clientFor(ctx, datasourceId, db)
	if err != nil {
		return err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	var expiration time.Duration
	if req.TTL > 0 {
		expiration = time.Duration(req.TTL) * time.Second
	}
	ok, err := client.SetNX(ctx, key, req.Value, expiration).Result()
	if err != nil {
		return wrapRedisErr(err)
	}
	if !ok {
		return apierrors.NewError(fmt.Errorf("key(%s) already exists", key), http.StatusConflict)
	}
	return nil
}

// DeleteKey 删除指定 key
func (c *controller) DeleteKey(ctx context.Context, datasourceId int64, db *int, key string) error {
	if err := requireRedisAdmin(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" {
		return apierrors.NewError(fmt.Errorf("key is required"), http.StatusBadRequest)
	}
	client, _, err := c.clientFor(ctx, datasourceId, db)
	if err != nil {
		return err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	n, err := client.Del(ctx, key).Result()
	if err != nil {
		return wrapRedisErr(err)
	}
	if n == 0 {
		return apierrors.NewError(fmt.Errorf("key not found"), http.StatusNotFound)
	}
	return nil
}

// DeleteKeys 批量删除 key，返回实际删除数量（为 0 不报错）
func (c *controller) DeleteKeys(ctx context.Context, datasourceId int64, db *int, keys []string) (*types.RedisDeleteKeysResult, error) {
	if err := requireRedisAdmin(ctx); err != nil {
		return nil, err
	}
	// 过滤空值并去重，保留 key 原始内容（不裁剪空白，空白可能是 key 的一部分）
	seen := make(map[string]struct{}, len(keys))
	cleaned := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, key)
	}
	if len(cleaned) == 0 {
		return nil, apierrors.NewError(fmt.Errorf("keys is required"), http.StatusBadRequest)
	}
	if len(cleaned) > maxBatchDeleteKeys {
		return nil, apierrors.NewError(fmt.Errorf("batch delete exceeds limit %d keys", maxBatchDeleteKeys), http.StatusBadRequest)
	}
	client, _, err := c.clientFor(ctx, datasourceId, db)
	if err != nil {
		return nil, err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	n, err := client.Del(ctx, cleaned...).Result()
	if err != nil {
		return nil, wrapRedisErr(err)
	}
	return &types.RedisDeleteKeysResult{Deleted: n}, nil
}

// UpdateKeyValue 修改 string 类型 key 的值并保持原 TTL；非 string 类型拒绝
func (c *controller) UpdateKeyValue(ctx context.Context, datasourceId int64, db *int, req *types.RedisUpdateKeyValueRequest) error {
	if err := requireRedisAdmin(ctx); err != nil {
		return err
	}
	if req == nil || strings.TrimSpace(req.Key) == "" {
		return apierrors.NewError(fmt.Errorf("key is required"), http.StatusBadRequest)
	}
	if runes := []rune(req.Value); int64(len(runes)) > maxWriteValueSize {
		return apierrors.NewError(fmt.Errorf("value length exceeds limit %d", maxWriteValueSize), http.StatusBadRequest)
	}
	client, _, err := c.clientFor(ctx, datasourceId, db)
	if err != nil {
		return err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	keyType, err := client.Type(ctx, req.Key).Result()
	if err != nil {
		return wrapRedisErr(err)
	}
	if keyType == "none" {
		return apierrors.NewError(fmt.Errorf("key not found"), http.StatusNotFound)
	}
	if keyType != "string" {
		return apierrors.NewError(fmt.Errorf("only string keys can be edited, current type: %s", keyType), http.StatusConflict)
	}

	// 超大 value 禁止经控制台编辑，避免截断内容误覆盖
	strLen, err := client.StrLen(ctx, req.Key).Result()
	if err != nil {
		return wrapRedisErr(err)
	}
	if strLen > maxWriteValueSize {
		return apierrors.NewError(
			fmt.Errorf("value too large to edit safely (size %d exceeds limit %d)", strLen, maxWriteValueSize),
			http.StatusConflict,
		)
	}

	// 写回时保持原 TTL（剩余 <=0 表示永不过期）
	var expiration time.Duration
	if ttl, err := client.TTL(ctx, req.Key).Result(); err == nil && ttl > 0 {
		expiration = ttl
	}
	if _, err = client.Set(ctx, req.Key, req.Value, expiration).Result(); err != nil {
		return wrapRedisErr(err)
	}
	return nil
}

// SetKeyTTL 修改 key 过期时间：ttl=-1 永久化（PERSIST）；ttl>=1 设置过期（EXPIRE）；
// ttl=0 拒绝（Redis EXPIRE key 0 会直接删除 key，防误删）
func (c *controller) SetKeyTTL(ctx context.Context, datasourceId int64, db *int, key string, ttl int64) error {
	if err := requireRedisAdmin(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" {
		return apierrors.NewError(fmt.Errorf("key is required"), http.StatusBadRequest)
	}
	if ttl < -1 || ttl == 0 || ttl > maxTTLSeconds {
		return apierrors.NewError(fmt.Errorf("ttl must be -1 (persist), 1..%d, or 0 is rejected to avoid accidental deletion", maxTTLSeconds), http.StatusBadRequest)
	}
	client, _, err := c.clientFor(ctx, datasourceId, db)
	if err != nil {
		return err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	var ok bool
	if ttl == -1 {
		ok, err = client.Persist(ctx, key).Result()
	} else {
		ok, err = client.Expire(ctx, key, time.Duration(ttl)*time.Second).Result()
	}
	if err != nil {
		return wrapRedisErr(err)
	}
	if !ok {
		return apierrors.NewError(fmt.Errorf("key not found"), http.StatusNotFound)
	}
	return nil
}

// readKeyValue 按类型读取 key 的值，统一做数量截断保护。
// string：用 GETRANGE + STRLEN，避免超大 key 全量入内存；截断上限与写入上限对齐。
func readKeyValue(ctx context.Context, client goredis.UniversalClient, key, keyType string) (interface{}, bool) {
	switch keyType {
	case "string":
		fullLen, err := client.StrLen(ctx, key).Result()
		if err != nil {
			return nil, false
		}
		// 按字节拉取，上限与写入一致；再按 rune 截断，避免截在多字节字符中间
		val, err := client.GetRange(ctx, key, 0, maxWriteValueSize-1).Result()
		if err != nil {
			return nil, false
		}
		truncated := int64(len(val)) < fullLen
		for len(val) > 0 && !utf8.ValidString(val) {
			val = val[:len(val)-1]
			truncated = true
		}
		if runes := []rune(val); len(runes) > maxWriteValueSize {
			val = string(runes[:maxWriteValueSize])
			truncated = true
		}
		return val, truncated

	case "hash":
		// 用 HScan 分批拉取，避免超大 hash 全量入内存；返回平铺的 [field, value, ...]
		fields, hCursor, err := client.HScan(ctx, key, 0, "*", maxCollectionSize).Result()
		if err != nil {
			return nil, false
		}
		truncated := hCursor != 0 || len(fields)/2 >= maxCollectionSize
		result := make(map[string]string, 0)
		for i := 0; i+1 < len(fields) && len(result) < maxCollectionSize; i += 2 {
			result[fields[i]] = truncateString(fields[i+1])
		}
		return result, truncated

	case "list":
		values, err := client.LRange(ctx, key, 0, maxCollectionSize-1).Result()
		if err != nil {
			return nil, false
		}
		return truncateSlice(values), int64(len(values)) >= maxCollectionSize

	case "set":
		// 用 SSCAN 只取首批，避免一次性加载超大集合
		members, cursor, err := client.SScan(ctx, key, 0, "", maxCollectionSize).Result()
		if err != nil {
			return nil, false
		}
		return truncateSlice(members), cursor != 0 || int64(len(members)) >= maxCollectionSize

	case "zset":
		items, err := client.ZRangeWithScores(ctx, key, 0, maxCollectionSize-1).Result()
		if err != nil {
			return nil, false
		}
		values := make([]string, 0, len(items))
		for _, item := range items {
			values = append(values, fmt.Sprintf("%v (score: %v)", item.Member, item.Score))
		}
		return values, int64(len(values)) >= maxCollectionSize

	default:
		// stream 等类型第一版不提供值预览
		return fmt.Sprintf("(%s 类型暂不支持预览)", keyType), false
	}
}

func truncateString(s string) string {
	if runes := []rune(s); len(runes) > maxStringValueSize {
		return string(runes[:maxStringValueSize])
	}
	return s
}

func truncateSlice(values []string) []string {
	for i := range values {
		values[i] = truncateString(values[i])
	}
	return values
}

// parseInfoField 从 INFO 原始输出中提取字段值
func parseInfoField(raw, field string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(strings.TrimSpace(line), "\r")
		if strings.HasPrefix(line, field+":") {
			return strings.TrimPrefix(line, field+":")
		}
	}
	return ""
}

func parseInfoInt(raw, field string) int64 {
	n, _ := strconv.ParseInt(parseInfoField(raw, field), 10, 64)
	return n
}

func parseInfoFloat(raw, field string) float64 {
	f, _ := strconv.ParseFloat(parseInfoField(raw, field), 64)
	return f
}

// keyspaceLineRe 匹配 INFO keyspace 段中的行：db0:keys=123,expires=45,avg_ttl=67890
var keyspaceLineRe = regexp.MustCompile(`^(db\d+):keys=(\d+),expires=(\d+),avg_ttl=(\d+)$`)

// parseKeyspaceDBs 从 INFO 原始输出中解析 keyspace 段，提取各 DB 的 key 分布
func parseKeyspaceDBs(raw string) []types.RedisKeyspaceDB {
	var dbs []types.RedisKeyspaceDB
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(strings.TrimSpace(line), "\r")
		m := keyspaceLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		keys, _ := strconv.ParseInt(m[2], 10, 64)
		expires, _ := strconv.ParseInt(m[3], 10, 64)
		avgTTL, _ := strconv.ParseInt(m[4], 10, 64)
		dbs = append(dbs, types.RedisKeyspaceDB{
			DB:      m[1],
			Keys:    keys,
			Expires: expires,
			AvgTTL:  avgTTL,
		})
	}
	return dbs
}
