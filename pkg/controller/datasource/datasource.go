/*
Copyright 2024 The Pixiu Authors.

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

package datasource

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Getter interface {
	Datasource() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateDatasourceRequest) error
	Update(ctx context.Context, req *types.UpdateDatasourceRequest) error
	Delete(ctx context.Context, datasourceId int64) error
	Get(ctx context.Context, datasourceId int64, options ...GetOption) (*types.Datasource, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)
}

type GetOption func(*getOptions)

type getOptions struct {
	redact bool
}

func WithCredentials() GetOption {
	return func(options *getOptions) {
		options.redact = false
	}
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

// validateDuplicate 校验同一 (cluster, type) 下的「默认实例唯一」与「同名」约束。
// excludeID 供 Update 排除记录自身（Create 传 0），保证「改名/改默认」不会与自身冲突。
func (c *controller) validateDuplicate(ctx context.Context, req *types.CreateDatasourceRequest, excludeID int64) error {
	datasources, err := c.factory.Datasource().List(
		ctx,
		db.WithClusterName(req.ClusterName),
		db.WithDatasourceType(req.Type),
	)
	if err != nil {
		klog.Errorf(
			"failed to list datasources for validation cluster=%s type=%d sub_type=%s name=%s: %v",
			req.ClusterName,
			req.Type,
			req.SubType,
			req.Name,
			err,
		)
		return apierrors.ErrServerInternal
	}

	for _, datasource := range datasources {
		if excludeID != 0 && datasource.Id == excludeID {
			continue
		}
		if req.IsDefault && datasource.IsDefault {
			return apierrors.NewError(
				fmt.Errorf("default datasource already exists for cluster=%s type=%d, please unset the current default first",
					req.ClusterName, req.Type),
				http.StatusConflict,
			)
		}

		if datasource.Name == req.Name {
			return apierrors.NewError(
				fmt.Errorf("datasource already exists: cluster=%s type=%d name=%s",
					req.ClusterName, req.Type, req.Name),
				http.StatusConflict,
			)
		}
	}

	return nil
}

func (c *controller) Create(ctx context.Context, req *types.CreateDatasourceRequest) error {
	// 请求体缺省 config 时 req.Config 为 nil：Clean 会解引用它并 panic（gin 的 Recovery 兜底成 500），
	// 这里显式拦截成 400，错误语义更清晰
	if req.Config == nil {
		return apierrors.NewError(fmt.Errorf("datasource config is required"), http.StatusBadRequest)
	}
	if err := validateRedisConstraint(req); err != nil {
		return err
	}
	if err := validatePostgresConstraint(req); err != nil {
		return err
	}
	if err := validateStorageConstraint(req, true); err != nil {
		return err
	}
	if err := c.validateDuplicate(ctx, req, 0); err != nil {
		return err
	}

	if !req.External && req.ClusterName == "" {
		return apierrors.NewError(
			fmt.Errorf("cluster_name is required when datasource is not external"),
			http.StatusBadRequest,
		)
	}

	// 对配置进行简化，移除不必要的配置
	req.Config.Clean(req.Type, req.SubType)
	cfg, err := req.Config.Marshal()
	if err != nil {
		return apierrors.NewError(fmt.Errorf("invalid datasource config: %v", err), http.StatusBadRequest)
	}

	_, err = c.factory.Datasource().Create(ctx, &model.Datasource{
		UserId:      req.UserId,
		ClusterName: req.ClusterName,
		Name:        req.Name,
		Type:        req.Type,
		SubType:     req.SubType,
		Config:      cfg,
		IsDefault:   req.IsDefault,
		External:    req.External,
		Description: req.Description,
	})
	if err != nil {
		klog.Errorf("failed to create datasource %s: %v", req.Name, err)
		return apierrors.ErrServerInternal
	}

	return nil
}

// validateRedisConstraint 校验 Redis 数据源的约束：
// 1. type 与 sub_type 必须配对（redis 可搭配缓存/中间件类型）；2. 仅支持外部直连；3. 必须提供连接地址
func validateRedisConstraint(req *types.CreateDatasourceRequest) error {
	isRedisType := req.Type == model.DatasourceTypeRedis
	isRedisSubType := req.SubType == model.DatasourceSubTypeRedis
	// 缓存类型仅与 redis 配对
	if isRedisType && !isRedisSubType {
		return apierrors.NewError(
			fmt.Errorf("cache datasource requires sub_type=%s", model.DatasourceSubTypeRedis),
			http.StatusBadRequest,
		)
	}
	// redis 允许搭配缓存（存量）或中间件类型
	if isRedisSubType && !isRedisType && req.Type != model.DatasourceTypeMiddleware {
		return apierrors.NewError(
			fmt.Errorf("redis datasource requires type=%d or type=%d", model.DatasourceTypeRedis, model.DatasourceTypeMiddleware),
			http.StatusBadRequest,
		)
	}
	if !isRedisSubType {
		return nil
	}
	if !req.External {
		return apierrors.NewError(
			fmt.Errorf("redis datasource only supports external direct connection, please enable external"),
			http.StatusBadRequest,
		)
	}
	if req.Config == nil || req.Config.Redis == nil {
		return apierrors.NewError(
			fmt.Errorf("redis datasource requires config.redis"),
			http.StatusBadRequest,
		)
	}

	redisCfg := req.Config.Redis
	// 归一化模式并按模式校验连接配置
	redisCfg.Mode = redisCfg.NormalizeMode()
	switch redisCfg.Mode {
	case types.RedisModeSentinel:
		if strings.TrimSpace(redisCfg.MasterName) == "" || len(redisCfg.Addresses) == 0 {
			return apierrors.NewError(
				fmt.Errorf("redis sentinel mode requires master_name and addresses"),
				http.StatusBadRequest,
			)
		}
	case types.RedisModeCluster:
		if len(redisCfg.Addresses) == 0 {
			return apierrors.NewError(
				fmt.Errorf("redis cluster mode requires addresses"),
				http.StatusBadRequest,
			)
		}
		redisCfg.DB = 0 // Redis Cluster 仅支持 db 0
	default:
		if strings.TrimSpace(redisCfg.Address) == "" {
			return apierrors.NewError(
				fmt.Errorf("redis standalone mode requires address (host:port)"),
				http.StatusBadRequest,
			)
		}
	}
	return nil
}

func validatePostgresConstraint(req *types.CreateDatasourceRequest) error {
	if req.SubType != model.DatasourceSubTypePostgres {
		return nil
	}
	if req.Type != model.DatasourceTypeMiddleware || !req.External {
		return apierrors.NewError(fmt.Errorf("postgres datasource requires middleware type and external direct connection"), http.StatusBadRequest)
	}
	if req.Config == nil || req.Config.Postgres == nil {
		return apierrors.NewError(fmt.Errorf("postgres datasource requires config.postgres"), http.StatusBadRequest)
	}
	cfg := req.Config.Postgres
	if strings.TrimSpace(cfg.Host) == "" || strings.TrimSpace(cfg.UserName) == "" {
		return apierrors.NewError(fmt.Errorf("postgres datasource requires host and user_name"), http.StatusBadRequest)
	}
	if cfg.Port <= 0 {
		cfg.Port = 5432
	}
	if strings.TrimSpace(cfg.Database) == "" {
		cfg.Database = "postgres"
	}
	if strings.TrimSpace(cfg.SSLMode) == "" {
		cfg.SSLMode = "prefer"
	}
	return nil
}

// validateStorageConstraint 校验对象存储数据源只能通过外部 HTTP 地址访问。
// 对象存储服务不对应 Pixiu 集群内的 Service，因此不支持内部数据源模式。
//
// 地址与 Access Key 也在这里前移校验：否则「创建成功、进对象管理才报错」的体验很差
// （storage.clientFor 在使用时才兜底，报错时机太晚）。
// requireSecret 为 false 时跳过 Secret Key 校验，供 Update 使用——编辑表单留空表示不修改。
func validateStorageConstraint(req *types.CreateDatasourceRequest, requireSecret bool) error {
	if req.SubType != model.DatasourceSubTypeStorage {
		return nil
	}
	if req.Type != model.DatasourceTypeMiddleware {
		return apierrors.NewError(
			fmt.Errorf("storage datasource requires type=%d", model.DatasourceTypeMiddleware),
			http.StatusBadRequest,
		)
	}
	if !req.External {
		return apierrors.NewError(
			fmt.Errorf("storage datasource only supports external direct connection, please enable external"),
			http.StatusBadRequest,
		)
	}
	if req.Config == nil || req.Config.Storage == nil {
		return apierrors.NewError(
			fmt.Errorf("storage datasource requires config.storage"),
			http.StatusBadRequest,
		)
	}

	sc := req.Config.Storage
	endpoint := strings.TrimSpace(sc.Endpoint)
	if endpoint == "" && req.Config.Log != nil {
		// 兼容存量数据：地址与凭据曾统一落在通用 log 配置里（clientFor 有同款回退）
		endpoint = strings.TrimSpace(req.Config.Log.URL)
	}
	if endpoint == "" {
		return apierrors.NewError(fmt.Errorf("storage endpoint is required"), http.StatusBadRequest)
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		return apierrors.NewError(
			fmt.Errorf("storage endpoint must start with http:// or https://"),
			http.StatusBadRequest,
		)
	}

	accessKey := strings.TrimSpace(sc.AccessKeyID)
	if accessKey == "" && req.Config.Log != nil {
		accessKey = strings.TrimSpace(req.Config.Log.UserName)
	}
	if accessKey == "" {
		return apierrors.NewError(fmt.Errorf("storage access key is required"), http.StatusBadRequest)
	}
	if requireSecret && strings.TrimSpace(sc.SecretAccessKey) == "" {
		if req.Config.Log == nil || req.Config.Log.Password == "" {
			return apierrors.NewError(fmt.Errorf("storage secret key is required"), http.StatusBadRequest)
		}
	}
	return nil
}

// 更新前置检查：资源存在
func (c *controller) preUpdate(ctx context.Context, id int64) (*model.Datasource, error) {
	old, err := c.factory.Datasource().Get(ctx, id)
	if err != nil {
		klog.Errorf("failed to get datasource(%d): %v", id, err)
		return nil, apierrors.ErrServerInternal
	}
	if old == nil {
		return nil, apierrors.NewError(fmt.Errorf("datasource not found"), http.StatusNotFound)
	}
	return old, nil
}

func (c *controller) Update(ctx context.Context, req *types.UpdateDatasourceRequest) error {
	if req.Config == nil {
		return apierrors.NewError(fmt.Errorf("datasource config is required"), http.StatusBadRequest)
	}
	if err := validateRedisConstraint(&req.CreateDatasourceRequest); err != nil {
		return err
	}
	if err := validatePostgresConstraint(&req.CreateDatasourceRequest); err != nil {
		return err
	}
	if err := validateStorageConstraint(&req.CreateDatasourceRequest, false); err != nil {
		return err
	}
	old, err := c.preUpdate(ctx, req.Id)
	if err != nil {
		klog.Errorf("pre-update check failed for datasource(%d): %v", req.Id, err)
		return err
	}

	// 资源级授权校验：超管放行 → 资源 owner 放行 → 角色 scope 命中放行
	if err = controllerutil.CheckResourceAccess(ctx, c.factory, old.UserId, types.ResourceTypeDatasource, req.Id); err != nil {
		return err
	}

	// 重名与默认实例唯一性校验，与 Create 复用同一份规则（排除自身，避免改名/改默认自冲突）
	if err = c.validateDuplicate(ctx, &req.CreateDatasourceRequest, req.Id); err != nil {
		return err
	}

	updates := make(map[string]interface{})

	// 读接口已脱敏：这里先用旧值回填本次留空/缺省的字段，再做 Clean，
	// 否则「只改描述」就会把密钥写成空（Update 是全量覆盖 config）
	var oldCfg types.DatasourceConfig
	if err = oldCfg.Unmarshal(old.Config); err != nil {
		klog.Errorf("failed to unmarshal datasource(%d) config: %v", req.Id, err)
		return apierrors.ErrServerInternal
	}
	req.Config.MergeSensitiveFields(&oldCfg)
	req.Config.Clean(req.Type, req.SubType)
	cfg, err := req.Config.Marshal()
	if err != nil {
		return err
	}
	if cfg != old.Config {
		updates["config"] = cfg
	}

	if req.Name != old.Name {
		updates["name"] = req.Name
	}
	if req.ClusterName != old.ClusterName {
		updates["cluster_name"] = req.ClusterName
	}
	if req.Type != old.Type {
		updates["type"] = req.Type
	}
	if req.SubType != old.SubType {
		updates["sub_type"] = req.SubType
	}
	if req.IsDefault != old.IsDefault {
		updates["is_default"] = req.IsDefault
	}
	if req.External != old.External {
		updates["external"] = req.External
	}
	if req.Description != old.Description {
		updates["description"] = req.Description
	}

	if len(updates) == 0 {
		klog.V(2).Infof("datasource(%d): no fields to update", req.Id)
		return nil
	}
	if err = c.factory.Datasource().Update(ctx, req.Id, req.ResourceVersion, updates); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(
				fmt.Errorf("datasource not found or resource version conflict"),
				http.StatusConflict,
			)
		}
		klog.Errorf("failed to update datasource %d: %v", req.Id, err)
		return apierrors.ErrServerInternal
	}

	return nil
}

func (c *controller) Delete(ctx context.Context, datasourceId int64) error {
	object, err := c.factory.Datasource().Get(ctx, datasourceId)
	if err != nil {
		klog.Errorf("failed to get datasource(%d): %v", datasourceId, err)
		return apierrors.ErrServerInternal
	}
	if object == nil {
		return apierrors.NewError(fmt.Errorf("datasource not found"), http.StatusNotFound)
	}
	if err = controllerutil.CheckResourceAccess(ctx, c.factory, object.UserId, types.ResourceTypeDatasource, datasourceId); err != nil {
		return err
	}
	if err := c.factory.Datasource().Delete(ctx, datasourceId); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("datasource not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to delete datasource %d: %v", datasourceId, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Get(ctx context.Context, datasourceId int64, options ...GetOption) (*types.Datasource, error) {
	getOptions := getOptions{redact: true}
	for _, option := range options {
		option(&getOptions)
	}

	object, err := c.factory.Datasource().Get(ctx, datasourceId)
	if err != nil {
		klog.Errorf("failed to get datasource(%d): %v", datasourceId, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("datasource not found"), http.StatusNotFound)
	}
	if err = controllerutil.CheckResourceAccess(ctx, c.factory, object.UserId, types.ResourceTypeDatasource, datasourceId); err != nil {
		return nil, err
	}
	ds, err := modelToTypeWithRedaction(object, getOptions.redact)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	return ds, nil
}

func (c *controller) List(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	// 资源级授权：非超管用户叠加 scope 授权的 datasource id（超管走 listOption.UserId 现状即可）
	authorizedDatasourceIDs, err := controllerutil.AuthorizedResourceIDs(ctx, c.factory, types.ResourceTypeDatasource)
	if err != nil {
		return nil, err
	}

	opts := []db.Options{
		db.WithNameLike(listOption.NameSelector),
	}

	// 集群数据源可见性：请求指定集群时，解析被授权集群（子集群）→ 主集群并校验集群访问权；
	// 否则走通用过滤（用户所属 + datasource scope 授权）
	if listOption.ClusterName != "" {
		clusterObj, err := c.factory.Cluster().GetBy(ctx, db.WithName(listOption.ClusterName))
		if err != nil {
			klog.Errorf("failed to get cluster %s for datasource list: %v", listOption.ClusterName, err)
			return nil, apierrors.ErrServerInternal
		}
		// 集群不存在或无访问权：返回空列表，避免泄露其他集群数据源
		if clusterObj == nil {
			return pageResult, nil
		}
		if err = controllerutil.CheckResourceAccess(ctx, c.factory, clusterObj.UserId, types.ResourceTypeCluster, clusterObj.Id); err != nil {
			return pageResult, nil
		}
		// 被授权集群是主集群的子集群，其监控数据源挂在主集群名下；列表仍返回主集群名（数据源归属真相）。
		// 代理权限由 AuthorizeClusterAccessByName 在主集群名下回落到子集群 scoped kubeconfig 解决。
		if clusterObj.OwnerReference != 0 {
			master, err := c.factory.Cluster().Get(ctx, clusterObj.OwnerReference)
			if err != nil || master == nil {
				klog.Warningf("cluster %s: master cluster(%d) not found, skipping datasource",
					listOption.ClusterName, clusterObj.OwnerReference)
				return pageResult, nil
			}
			listOption.ClusterName = master.Name
		}
		opts = append(opts, db.WithClusterName(listOption.ClusterName))
	} else {
		opts = append(opts, db.WithUserOrResourceIDs(listOption.UserId, authorizedDatasourceIDs))
	}
	if listOption.DatasourceType != nil {
		opts = append(opts, db.WithDatasourceType(*listOption.DatasourceType))
	}
	if listOption.SubType != "" {
		opts = append(opts, db.WithDatasourceSubType(listOption.SubType))
	}

	pageResult.Total, err = c.factory.Datasource().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count dataSources: %v", err)
		pageResult.Message = err.Error()
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, []db.Options{
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	}...)

	objects, err := c.factory.Datasource().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list datasources: %v", err)
		pageResult.Message = err.Error()
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.Datasource, 0)
	for i := range objects {
		t, convErr := modelToTypeWithRedaction(&objects[i], true)
		if convErr != nil {
			return nil, apierrors.ErrServerInternal
		}
		items = append(items, *t)
	}
	pageResult.Items = items

	return pageResult, nil
}

func modelToTypeWithRedaction(object *model.Datasource, redact bool) (*types.Datasource, error) {
	var cfg types.DatasourceConfig
	if err := cfg.Unmarshal(object.Config); err != nil {
		return nil, err
	}
	if redact {
		redactDatasourceConfig(&cfg)
	}
	return &types.Datasource{
		PixiuMeta: types.PixiuMeta{
			Id:              object.Id,
			ResourceVersion: object.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   object.GmtCreate,
			GmtModified: object.GmtModified,
		},
		UserId:      object.UserId,
		ClusterName: object.ClusterName,
		Name:        object.Name,
		Type:        object.Type,
		SubType:     object.SubType,
		Config:      cfg,
		IsDefault:   object.IsDefault,
		External:    object.External,
		Description: object.Description,
	}, nil
}

// redactDatasourceConfig removes credentials before a datasource is returned by an API.
// The database keeps the original configuration for connection and update workflows.
func redactDatasourceConfig(cfg *types.DatasourceConfig) {
	if cfg == nil {
		return
	}
	if cfg.Log != nil {
		cfg.Log.Password = ""
	}
	if cfg.Alert != nil {
		cfg.Alert.Password = ""
	}
	if cfg.Redis != nil {
		cfg.Redis.Password = ""
		cfg.Redis.SentinelPassword = ""
	}
	if cfg.Mysql != nil {
		cfg.Mysql.Password = ""
	}
	if cfg.Postgres != nil {
		cfg.Postgres.Password = ""
	}
	if cfg.Storage != nil {
		cfg.Storage.SecretAccessKey = ""
		cfg.Storage.SessionToken = ""
	}
}

// preserveDatasourcePasswords keeps existing credentials when an update omits them.
// Datasource API responses redact secrets, so an unchanged frontend form submits empty values.
func preserveDatasourcePasswords(cfg *types.DatasourceConfig, oldConfig string) error {
	if cfg == nil {
		return nil
	}
	var old types.DatasourceConfig
	if err := old.Unmarshal(oldConfig); err != nil {
		return err
	}
	if cfg.Log != nil && old.Log != nil && cfg.Log.Password == "" {
		cfg.Log.Password = old.Log.Password
	}
	if cfg.Alert != nil && old.Alert != nil && cfg.Alert.Password == "" {
		cfg.Alert.Password = old.Alert.Password
	}
	if cfg.Redis != nil && old.Redis != nil {
		if cfg.Redis.Password == "" {
			cfg.Redis.Password = old.Redis.Password
		}
		if cfg.Redis.SentinelPassword == "" {
			cfg.Redis.SentinelPassword = old.Redis.SentinelPassword
		}
	}
	if cfg.Mysql != nil && old.Mysql != nil && cfg.Mysql.Password == "" {
		cfg.Mysql.Password = old.Mysql.Password
	}
	if cfg.Postgres != nil && old.Postgres != nil && cfg.Postgres.Password == "" {
		cfg.Postgres.Password = old.Postgres.Password
	}
	return nil
}
