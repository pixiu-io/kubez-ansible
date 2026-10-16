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

// Package mysql 提供 MySQL 中间件管理能力（外部直连），
// 覆盖实例概览、库表浏览、SQL 控制台、用户权限、会话、慢查询与备份
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
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
	mysqlDialTimeout  = 5 * time.Second
	mysqlOpTimeout    = 10 * time.Second
	mysqlQueryTimeout = 30 * time.Second // SQL 控制台执行上限

	maxCachedConns      = 64   // 连接池缓存上限，超过时按最近访问时间清理最旧连接
	maxSessionInfo      = 2048 // PROCESSLIST Info 字段截断长度（rune）
	maxSlowPageSize     = 100  // 慢查询分页单页上限
	defaultSlowPageSize = 20   // 慢查询缺省页大小

	// 系统库：概览统计与浏览时排除
	systemDatabases = "'information_schema','mysql','performance_schema','sys'"
)

// allowedDSNParams 数据源附加 DSN 参数白名单，防止注入任意连接参数
var allowedDSNParams = map[string]struct{}{
	"charset": {}, "collation": {}, "parseTime": {}, "loc": {},
	"timeout": {}, "readTimeout": {}, "writeTimeout": {},
	"tls": {}, "interpolateParams": {}, "allowNativePasswords": {},
}

type Interface interface {
	Ping(ctx context.Context, datasourceId int64) (*types.MySQLPing, error)
	// PingAdhoc 临时探测（不落库、不缓存连接），用于创建数据源前的连通性验证；仅管理员可调用
	PingAdhoc(ctx context.Context, datasourceId int64, cfg *types.MySQLSourceConfig) (*types.MySQLPing, error)
	Info(ctx context.Context, datasourceId int64) (*types.MySQLServerInfo, error)
	ListDatabases(ctx context.Context, datasourceId int64) ([]types.MySQLDatabase, error)
	ListTables(ctx context.Context, datasourceId int64, database string) ([]types.MySQLTable, error)
	GetTableDetail(ctx context.Context, datasourceId int64, database, table string) (*types.MySQLTableDetail, error)
	// ExecuteSQL SQL 控制台：读语句放行，写语句仅管理员可执行
	ExecuteSQL(ctx context.Context, datasourceId int64, req *types.MySQLQueryRequest) (*types.MySQLQueryResult, error)
	// ExecuteBatchSQL SQL 控制台批量执行：服务端拆分语句并逐条执行，遇错即停，错误带语句序号
	ExecuteBatchSQL(ctx context.Context, datasourceId int64, req *types.MySQLExecuteBatchRequest) (*types.MySQLExecuteBatchResult, error)
	// 用户权限管理（均仅管理员可调用）
	ListUsers(ctx context.Context, datasourceId int64) ([]types.MySQLUser, error)
	CreateUser(ctx context.Context, datasourceId int64, req *types.MySQLCreateUserRequest) error
	DeleteUser(ctx context.Context, datasourceId int64, user, host string) error
	GrantUser(ctx context.Context, datasourceId int64, req *types.MySQLGrantRequest) error
	// 会话管理
	ListSessions(ctx context.Context, datasourceId int64) ([]types.MySQLSession, error)
	// KillSession 终止会话，仅管理员可调用
	KillSession(ctx context.Context, datasourceId int64, id int64) error
	ListSlowQueries(ctx context.Context, datasourceId, page, pageSize int64) (*types.MySQLSlowQueryList, error)
	// Backup 生成库/表的 SQL 文本备份，仅管理员可调用
	Backup(ctx context.Context, datasourceId int64, req *types.MySQLBackupRequest) (*types.MySQLBackupResult, error)
	// ExportTableStreaming 流式导出表数据为 CSV，需要 gin.Context 以便直接写入响应体；仅管理员可调用
	ExportTableStreaming(c *gin.Context, datasourceId int64, req *types.MySQLTableExportRequest) error
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory

	mu    sync.Mutex
	conns map[string]*cachedConn
}

// cachedConn 按数据源缓存连接池，配置变化（resourceVersion/指纹）时重建
type cachedConn struct {
	resourceVersion int64
	fingerprint     string
	db              *sql.DB
	lastAccess      time.Time
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{
		cc:      cfg,
		factory: f,
		conns:   make(map[string]*cachedConn),
	}
}

// requireMySQLAdmin 限制高危操作（临时探测 / 写 SQL / 用户管理 / 会话终止 / 备份）
// 仅超管与内置管理员可执行，与菜单 AdminOnly（IsRoot / IsAdmin）对齐
func requireMySQLAdmin(ctx context.Context) error {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return err
	}
	if user.Role != model.RoleRoot && user.Role != model.RoleAdmin {
		return apierrors.ErrForbidden
	}
	return nil
}

// validateMySQLConfig 校验连接配置并归一化缺省值
func validateMySQLConfig(cfg *types.MySQLSourceConfig) error {
	if cfg == nil {
		return fmt.Errorf("mysql config is required")
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	if cfg.Host == "" {
		return fmt.Errorf("mysql host is required")
	}
	cfg.Port = cfg.NormalizePort()
	if strings.TrimSpace(cfg.UserName) == "" {
		return fmt.Errorf("mysql user_name is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = int(mysqlDialTimeout / time.Second)
	}
	return nil
}

// buildDSN 组装连接串；附加参数经白名单校验，防止注入任意 DSN 参数
func buildDSN(cfg *types.MySQLSourceConfig) (string, error) {
	if err := validateMySQLConfig(cfg); err != nil {
		return "", apierrors.NewError(err, http.StatusBadRequest)
	}
	charset := strings.TrimSpace(cfg.Charset)
	if charset == "" {
		charset = "utf8mb4"
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=%ds&charset=%s&parseTime=true",
		cfg.UserName, cfg.Password, cfg.Host, cfg.Port, cfg.Database, cfg.Timeout, charset)

	extra := strings.TrimSpace(cfg.Params)
	if extra == "" {
		return dsn, nil
	}
	for _, kv := range strings.Split(extra, "&") {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return "", apierrors.NewError(fmt.Errorf("invalid mysql dsn param: %s", kv), http.StatusBadRequest)
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if _, ok := allowedDSNParams[key]; !ok {
			return "", apierrors.NewError(fmt.Errorf("mysql dsn param not allowed: %s", key), http.StatusBadRequest)
		}
		// 参数值只允许安全字符，杜绝二次注入
		if value == "" || strings.ContainsAny(value, "& \t\r\n'\"") {
			return "", apierrors.NewError(fmt.Errorf("invalid mysql dsn param value for %s", key), http.StatusBadRequest)
		}
		dsn += "&" + key + "=" + value
	}
	return dsn, nil
}

// openMySQL 建立连接池并验证连通性（调用方负责 Close）
func openMySQL(cfg *types.MySQLSourceConfig) (*sql.DB, error) {
	dsn, err := buildDSN(cfg)
	if err != nil {
		return nil, err
	}
	dbConn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("failed to open mysql connection: %v", err), http.StatusBadGateway)
	}
	dbConn.SetMaxOpenConns(10)
	dbConn.SetMaxIdleConns(2)
	dbConn.SetConnMaxIdleTime(5 * time.Minute)
	dbConn.SetConnMaxLifetime(30 * time.Minute)
	return dbConn, nil
}

// configFingerprint 配置摘要，用于缓存失效判断
func configFingerprint(cfg *types.MySQLSourceConfig) string {
	return fmt.Sprintf("%s|%d|%s|%s|%s|%s|%s",
		cfg.Host, cfg.Port, cfg.UserName, cfg.Password, cfg.Database, cfg.Charset, cfg.Params)
}

// connFor 获取数据源对应的连接池（带缓存、配置变更失效与归属校验）
func (c *controller) connFor(ctx context.Context, datasourceId int64) (*sql.DB, *types.MySQLSourceConfig, error) {
	object, err := c.factory.Datasource().Get(ctx, datasourceId)
	if err != nil {
		klog.Errorf("failed to get datasource(%d): %v", datasourceId, err)
		return nil, nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, nil, apierrors.NewError(fmt.Errorf("datasource not found"), http.StatusNotFound)
	}
	if object.Type != model.DatasourceTypeMiddleware ||
		object.SubType != model.DatasourceSubTypeMySQL {
		return nil, nil, apierrors.NewError(fmt.Errorf("datasource(%d) is not a mysql datasource", datasourceId), http.StatusBadRequest)
	}
	if !object.External {
		return nil, nil, apierrors.NewError(fmt.Errorf("mysql datasource only supports external direct connection"), http.StatusBadRequest)
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
	if cfg.Mysql == nil {
		return nil, nil, apierrors.NewError(fmt.Errorf("datasource(%d) missing mysql config", datasourceId), http.StatusBadRequest)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	cacheKey := strconv.FormatInt(datasourceId, 10)
	fingerprint := configFingerprint(cfg.Mysql)
	if cached, ok := c.conns[cacheKey]; ok &&
		cached.resourceVersion == object.ResourceVersion &&
		cached.fingerprint == fingerprint {
		cached.lastAccess = time.Now()
		return cached.db, cfg.Mysql, nil
	}

	// 配置变更或首次连接：关闭旧连接池并重建
	if cached, ok := c.conns[cacheKey]; ok {
		_ = cached.db.Close()
		delete(c.conns, cacheKey)
	}

	dbConn, err := openMySQL(cfg.Mysql)
	if err != nil {
		return nil, nil, err
	}
	c.conns[cacheKey] = &cachedConn{
		resourceVersion: object.ResourceVersion,
		fingerprint:     fingerprint,
		db:              dbConn,
		lastAccess:      time.Now(),
	}
	// 连接缓存上限保护：超过 maxCachedConns 时清理最近访问时间最旧的连接池，避免无限增长
	if len(c.conns) > maxCachedConns {
		var oldestKey string
		var oldest time.Time
		for k, cc := range c.conns {
			if oldestKey == "" || cc.lastAccess.Before(oldest) {
				oldestKey, oldest = k, cc.lastAccess
			}
		}
		if oldestKey != "" {
			_ = c.conns[oldestKey].db.Close()
			delete(c.conns, oldestKey)
		}
	}
	return dbConn, cfg.Mysql, nil
}

// wrapMySQLErr 将 MySQL 错误转换为带语义错误码的网关类错误：
// 认证失败→401；连接层故障→502；权限不足→403；其余→502 携带原始信息
func wrapMySQLErr(err error) error {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "access denied") || strings.Contains(msg, "authentication"):
		return apierrors.NewError(fmt.Errorf("mysql authentication failed: %v", err), http.StatusUnauthorized)
	case strings.Contains(msg, "command denied"):
		return apierrors.NewError(fmt.Errorf("mysql permission denied: %v", err), http.StatusForbidden)
	case strings.Contains(msg, "connection refused") || strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "invalid connection") || strings.Contains(msg, "driver: bad connection") ||
		strings.Contains(msg, "no such host") || strings.Contains(msg, "network is unreachable") ||
		strings.Contains(msg, "unknown database"):
		return apierrors.NewError(fmt.Errorf("mysql connection failed: %v", err), http.StatusBadGateway)
	default:
		return apierrors.NewError(fmt.Errorf("mysql request failed: %v", err), http.StatusBadGateway)
	}
}

func opContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, mysqlOpTimeout)
}

func queryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, mysqlQueryTimeout)
}

// validIdentifier 标识符（库名/表名）合法性：长度 1-64，不含反引号与 NUL；
// 拼接 SQL 时统一经 quoteIdent 用反引号包裹并转义，杜绝注入
func validIdentifier(s string) bool {
	return s != "" && len(s) <= 64 && !strings.ContainsAny(s, "`\x00")
}

// quoteIdent 反引号包裹标识符，内部反引号翻倍转义
func quoteIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

// checkIdentifier 校验标识符，非法时返回 400
func checkIdentifier(kind, s string) error {
	if !validIdentifier(s) {
		return apierrors.NewError(fmt.Errorf("invalid %s name: %q", kind, s), http.StatusBadRequest)
	}
	return nil
}

func (c *controller) Ping(ctx context.Context, datasourceId int64) (*types.MySQLPing, error) {
	dbConn, cfg, err := c.connFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}
	return pingMySQL(ctx, dbConn, cfg)
}

// PingAdhoc 使用临时连接探测指定配置，探测完成后立即关闭连接。
// 仅管理员可调用：请求体可指定任意地址，属于认证后 SSRF 面，必须收敛权限。
// datasourceId > 0 时读取已保存的数据源，并在密码为空时补齐已保存密码。
func (c *controller) PingAdhoc(ctx context.Context, datasourceId int64, cfg *types.MySQLSourceConfig) (*types.MySQLPing, error) {
	if err := requireMySQLAdmin(ctx); err != nil {
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
		if err = saved.Unmarshal(object.Config); err != nil || saved.Mysql == nil {
			return nil, apierrors.NewError(fmt.Errorf("invalid MySQL datasource config"), http.StatusBadRequest)
		}
		if cfg == nil {
			cfg = &types.MySQLSourceConfig{}
		}
		if cfg.Password == "" {
			cfg.Password = saved.Mysql.Password
		}
	}

	dbConn, err := openMySQL(cfg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dbConn.Close() }()
	return pingMySQL(ctx, dbConn, cfg)
}

// pingMySQL 执行 PING + 顺带取版本（失败不影响探测结果）
func pingMySQL(ctx context.Context, dbConn *sql.DB, cfg *types.MySQLSourceConfig) (*types.MySQLPing, error) {
	result := &types.MySQLPing{Address: cfg.DisplayAddress()}
	ctx, cancel := opContext(ctx)
	defer cancel()

	start := time.Now()
	if err := dbConn.PingContext(ctx); err != nil {
		result.Message = err.Error()
		return result, nil
	}
	result.Connected = true
	result.LatencyMs = time.Since(start).Milliseconds()

	var version string
	if err := dbConn.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err == nil {
		result.Version = version
	}
	return result, nil
}

// scanStatusVars 查询 GLOBAL STATUS 中的指定变量，返回 map
func scanStatusVars(ctx context.Context, dbConn *sql.DB, names ...string) (map[string]string, error) {
	query := fmt.Sprintf("SHOW GLOBAL STATUS WHERE Variable_name IN ('%s')", strings.Join(names, "','"))
	rows, err := dbConn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		result[name] = value
	}
	return result, rows.Err()
}

// scanVars 查询 GLOBAL VARIABLES 中的指定变量，返回 map
func scanVars(ctx context.Context, dbConn *sql.DB, names ...string) (map[string]string, error) {
	query := fmt.Sprintf("SHOW GLOBAL VARIABLES WHERE Variable_name IN ('%s')", strings.Join(names, "','"))
	rows, err := dbConn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		result[name] = value
	}
	return result, rows.Err()
}

func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// Info 实例概览：版本、连接数、QPS 估算、慢查询配置与数据体积
func (c *controller) Info(ctx context.Context, datasourceId int64) (*types.MySQLServerInfo, error) {
	dbConn, _, err := c.connFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	info := &types.MySQLServerInfo{}
	if err := dbConn.QueryRowContext(ctx, "SELECT VERSION()").Scan(&info.Version); err != nil {
		return nil, wrapMySQLErr(err)
	}

	status, err := scanStatusVars(ctx, dbConn,
		"Uptime", "Threads_connected", "Questions", "Slow_queries",
		"Innodb_rows_read", "Bytes_received", "Bytes_sent")
	if err != nil {
		return nil, wrapMySQLErr(err)
	}
	info.UptimeInSeconds = parseInt64(status["Uptime"])
	info.ThreadsConnected = parseInt64(status["Threads_connected"])
	info.QuestionCount = parseInt64(status["Questions"])
	info.SlowQueries = parseInt64(status["Slow_queries"])
	info.InnoDBRowsRead = parseInt64(status["Innodb_rows_read"])
	info.BytesReceived = parseInt64(status["Bytes_received"])
	info.BytesSent = parseInt64(status["Bytes_sent"])
	if info.UptimeInSeconds > 0 {
		info.QueriesPerSecond = info.QuestionCount / info.UptimeInSeconds
	}

	vars, err := scanVars(ctx, dbConn,
		"max_connections", "read_only", "slow_query_log", "long_query_time", "slow_query_log_file")
	if err != nil {
		return nil, wrapMySQLErr(err)
	}
	info.MaxConnections = parseInt64(vars["max_connections"])
	info.ReadOnly = strings.EqualFold(vars["read_only"], "ON") || vars["read_only"] == "1"
	info.SlowQueryLog = vars["slow_query_log"]
	info.LongQueryTime = vars["long_query_time"]
	info.SlowQueryLogFile = vars["slow_query_log_file"]

	// 用户库数量与总体积（information_schema 统计，失败不阻断概览）
	var dbCount int
	if err := dbConn.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME NOT IN ("+systemDatabases+")").
		Scan(&dbCount); err == nil {
		info.DatabaseCount = dbCount
	}
	var sizeBytes sql.NullInt64
	if err := dbConn.QueryRowContext(ctx,
		"SELECT SUM(DATA_LENGTH + INDEX_LENGTH) FROM information_schema.TABLES WHERE TABLE_SCHEMA NOT IN ("+systemDatabases+")").
		Scan(&sizeBytes); err == nil && sizeBytes.Valid {
		info.DataSizeMB = sizeBytes.Int64 / 1024 / 1024
	}
	return info, nil
}

// ListDatabases 用户库列表（含字符集、体积、表数），排除系统库
func (c *controller) ListDatabases(ctx context.Context, datasourceId int64) ([]types.MySQLDatabase, error) {
	dbConn, _, err := c.connFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	rows, err := dbConn.QueryContext(ctx, `
SELECT s.SCHEMA_NAME, s.DEFAULT_CHARACTER_SET_NAME, s.DEFAULT_COLLATION_NAME,
       IFNULL(t.size_bytes, 0), IFNULL(t.table_num, 0)
FROM information_schema.SCHEMATA s
LEFT JOIN (
    SELECT TABLE_SCHEMA, SUM(DATA_LENGTH + INDEX_LENGTH) AS size_bytes, COUNT(*) AS table_num
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA NOT IN (`+systemDatabases+`)
    GROUP BY TABLE_SCHEMA
) t ON t.TABLE_SCHEMA = s.SCHEMA_NAME
WHERE s.SCHEMA_NAME NOT IN (`+systemDatabases+`)
ORDER BY s.SCHEMA_NAME`)
	if err != nil {
		return nil, wrapMySQLErr(err)
	}
	defer rows.Close()

	databases := make([]types.MySQLDatabase, 0)
	for rows.Next() {
		var item types.MySQLDatabase
		var sizeBytes int64
		if err := rows.Scan(&item.Name, &item.Charset, &item.Collation, &sizeBytes, &item.TableNum); err != nil {
			return nil, wrapMySQLErr(err)
		}
		item.SizeMB = sizeBytes / 1024 / 1024
		databases = append(databases, item)
	}
	return databases, wrapRowsErr(rows.Err())
}

// ListTables 指定库的表概览（引擎、估算行数、体积）
func (c *controller) ListTables(ctx context.Context, datasourceId int64, database string) ([]types.MySQLTable, error) {
	if err := checkIdentifier("database", database); err != nil {
		return nil, err
	}
	dbConn, _, err := c.connFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	rows, err := dbConn.QueryContext(ctx, `
SELECT TABLE_NAME, IFNULL(ENGINE, ''), IFNULL(TABLE_ROWS, 0),
       IFNULL(DATA_LENGTH + INDEX_LENGTH, 0), CREATE_TIME, IFNULL(TABLE_COMMENT, '')
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = ?
ORDER BY TABLE_NAME`, database)
	if err != nil {
		return nil, wrapMySQLErr(err)
	}
	defer rows.Close()

	tables := make([]types.MySQLTable, 0)
	for rows.Next() {
		var item types.MySQLTable
		var sizeBytes int64
		var createTime sql.NullTime
		if err := rows.Scan(&item.Name, &item.Engine, &item.Rows, &sizeBytes, &createTime, &item.Comment); err != nil {
			return nil, wrapMySQLErr(err)
		}
		item.SizeMB = float64(sizeBytes) / 1024 / 1024
		if createTime.Valid {
			item.CreateTime = createTime.Time.Format("2006-01-02 15:04:05")
		}
		tables = append(tables, item)
	}
	return tables, wrapRowsErr(rows.Err())
}

// GetTableDetail 表详情：DDL、列元数据、索引
func (c *controller) GetTableDetail(ctx context.Context, datasourceId int64, database, table string) (*types.MySQLTableDetail, error) {
	if err := checkIdentifier("database", database); err != nil {
		return nil, err
	}
	if err := checkIdentifier("table", table); err != nil {
		return nil, err
	}
	dbConn, _, err := c.connFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	detail := &types.MySQLTableDetail{Name: table}

	// SHOW CREATE TABLE（标识符经 quoteIdent 转义，不可参数化）
	var tableName, ddl string
	if err := dbConn.QueryRowContext(ctx,
		"SHOW CREATE TABLE "+quoteIdent(database)+"."+quoteIdent(table)).
		Scan(&tableName, &ddl); err != nil {
		return nil, wrapMySQLErr(err)
	}
	detail.DDL = ddl

	// 表元信息（引擎/估算行数/注释），查询失败不影响主流程
	var engine, comment string
	var estRows int64
	if err := dbConn.QueryRowContext(ctx,
		"SELECT IFNULL(ENGINE, ''), IFNULL(TABLE_COMMENT, ''), IFNULL(TABLE_ROWS, 0) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?",
		database, table).Scan(&engine, &comment, &estRows); err == nil {
		detail.Engine = engine
		detail.Comment = comment
		detail.Rows = estRows
	}

	// 列元数据
	colRows, err := dbConn.QueryContext(ctx, `
SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, IFNULL(COLUMN_KEY, ''),
       IFNULL(COLUMN_DEFAULT, ''), EXTRA, IFNULL(COLUMN_COMMENT, ''), ORDINAL_POSITION
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
ORDER BY ORDINAL_POSITION`, database, table)
	if err != nil {
		return nil, wrapMySQLErr(err)
	}
	defer colRows.Close()

	for colRows.Next() {
		var col types.MySQLColumn
		if err := colRows.Scan(&col.Name, &col.Type, &col.Null, &col.Key,
			&col.Default, &col.Extra, &col.Comment, &col.OrdinalPos); err != nil {
			return nil, wrapMySQLErr(err)
		}
		detail.Columns = append(detail.Columns, col)
	}
	if err := colRows.Err(); err != nil {
		return nil, wrapMySQLErr(err)
	}

	// 索引（SHOW INDEX 结果按索引名聚合）
	idxRows, err := dbConn.QueryContext(ctx,
		"SHOW INDEX FROM "+quoteIdent(database)+"."+quoteIdent(table))
	if err != nil {
		return nil, wrapMySQLErr(err)
	}
	defer idxRows.Close()

	indexes := make([]types.MySQLIndex, 0)
	idxPos := make(map[string]int)
	idxCols, err := idxRows.Columns()
	if err != nil {
		return nil, wrapMySQLErr(err)
	}
	for idxRows.Next() {
		values := make([]interface{}, len(idxCols))
		ptrs := make([]interface{}, len(idxCols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := idxRows.Scan(ptrs...); err != nil {
			return nil, wrapMySQLErr(err)
		}
		// 结果列：Table, Non_unique, Key_name, Seq_in_index, Column_name, ..., Index_type, Comment, Index_comment
		asString := func(i int) string {
			if i >= len(values) {
				return ""
			}
			switch v := values[i].(type) {
			case []byte:
				return string(v)
			case string:
				return v
			case nil:
				return ""
			default:
				return fmt.Sprintf("%v", v)
			}
		}
		keyName := asString(2)
		column := asString(4)
		nonUnique := asString(1) == "1"
		idxType := ""
		if len(idxCols) > 10 {
			idxType = asString(10)
		}
		if pos, ok := idxPos[keyName]; ok {
			indexes[pos].Columns += "," + column
		} else {
			idxPos[keyName] = len(indexes)
			indexes = append(indexes, types.MySQLIndex{
				Name:      keyName,
				NonUnique: nonUnique,
				Columns:   column,
				Type:      idxType,
			})
		}
	}
	if err := idxRows.Err(); err != nil {
		return nil, wrapMySQLErr(err)
	}
	detail.Indexes = indexes
	return detail, nil
}

func wrapRowsErr(err error) error {
	if err == nil {
		return nil
	}
	return wrapMySQLErr(err)
}
