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

// Package apps 实现基于 Helm Chart 的应用商店：chart 目录管理 + 安装编排。
package apps

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/chartstore"
	"github.com/caoyingjunz/pixiu/pkg/controller/helm"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type Getter interface {
	Appstore() Interface
}

type Interface interface {
	// chart 目录
	Upload(ctx context.Context, filename, category string, r io.Reader) (*types.ChartInfo, error)
	ImportFromURL(ctx context.Context, req *types.ImportChartRequest) (*types.ChartInfo, error)
	List(ctx context.Context, opt types.ListChartOption) (*types.ChartListResponse, error)
	Get(ctx context.Context, id int64) (*types.ChartInfo, error)
	Delete(ctx context.Context, id int64) error
	DefaultValues(ctx context.Context, id int64, version string) (string, error)
	Templates(ctx context.Context, id int64) ([]types.ChartTemplateInfo, error)
	TemplateContent(ctx context.Context, id int64, name string) (string, error)

	// 安装编排（release 以 (cluster, namespace, name) 集群作用域自然身份定位；异步：入队即返，后台 worker 执行）
	Install(ctx context.Context, cluster, namespace string, req *types.InstallReleaseRequest) (*types.InstallInfo, error)
	ListReleases(ctx context.Context, cluster, namespace string) ([]types.InstallInfo, error)
	GetRelease(ctx context.Context, cluster, namespace, name string) (*types.InstallInfo, error)
	Upgrade(ctx context.Context, cluster, namespace, name string, req *types.UpgradeChartRequest) error
	Uninstall(ctx context.Context, cluster, namespace, name string) error
	Rollback(ctx context.Context, cluster, namespace, name string, req *types.RollbackChartRequest) error
	History(ctx context.Context, cluster, namespace, name string) ([]types.InstallHistoryItem, error)

	// Run 启动异步编排 worker 池（由 server 启动阶段挂载）。
	Run(ctx context.Context, workers int) error
	// WatchLogs SSE 实时推送单条安装的执行日志。
	WatchLogs(ctx context.Context, cluster, namespace, name string, w http.ResponseWriter, r *http.Request) error
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
	helmSvc helm.Interface
	store   chartstore.Store

	// storeErr 存储后端初始化失败时记录，调用时优先返回，避免服务启动即失败。
	storeErr error
}

var _ Interface = &controller{}

// New 构造应用商店 controller。存储后端初始化失败不 panic，延迟到具体调用时报错。
func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	store, err := chartstore.New(cfg.Appstore)
	return &controller{
		cc:       cfg,
		factory:  f,
		helmSvc:  helm.NewHelm(f),
		store:    store,
		storeErr: err,
	}
}

func (c *controller) ensureStore() error {
	if c.storeErr != nil {
		return fmt.Errorf("appstore storage not available: %w", c.storeErr)
	}
	if c.store == nil {
		return fmt.Errorf("appstore storage not initialized")
	}
	return nil
}

// storeKey chart 包在存储后端中的对象键。
func storeKey(name, version string) string {
	return fmt.Sprintf("charts/%s-%s.tgz", name, version)
}

// chartFor 读取 chart 目录项与版本列表，chart 不存在时返回 nil。
func (c *controller) chartFor(ctx context.Context, id int64) (*model.Chart, []model.AppstoreChartVersion, error) {
	chart, err := c.factory.Chart().GetChart(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if chart == nil {
		return nil, nil, nil
	}
	versions, err := c.factory.Chart().ListChartVersions(ctx, chart.Id)
	if err != nil {
		return nil, nil, err
	}
	return chart, versions, nil
}
