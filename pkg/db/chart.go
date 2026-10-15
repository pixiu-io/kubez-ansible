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

package db

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

// ChartInterface 应用商店三张表（charts / chart_versions / appstore_installs）的数据访问。
type ChartInterface interface {
	// Chart 目录
	CreateChart(ctx context.Context, object *model.Chart) (*model.Chart, error)
	UpdateChart(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	GetChart(ctx context.Context, id int64) (*model.Chart, error)
	GetChartByName(ctx context.Context, name string) (*model.Chart, error)
	ListCharts(ctx context.Context, opts ...Options) ([]model.Chart, error)
	CountCharts(ctx context.Context, opts ...Options) (int64, error)
	DeleteChart(ctx context.Context, id int64) error
	// IncrementInstallCount 累计安装次数 +1（原子自增，安装成功时调用）。
	IncrementInstallCount(ctx context.Context, chartId int64) error
	// ListInstalledChartIds 返回给定 chart 中「存在至少一条安装记录」的 chart_id 集合。
	// 批量一次查询，供列表判定 installed 字段，避免 N+1。
	ListInstalledChartIds(ctx context.Context, chartIds []int64) ([]int64, error)

	// Chart 版本
	CreateChartVersion(ctx context.Context, object *model.AppstoreChartVersion) (*model.AppstoreChartVersion, error)
	GetChartVersion(ctx context.Context, chartId int64, version string) (*model.AppstoreChartVersion, error)
	ListChartVersions(ctx context.Context, chartId int64) ([]model.AppstoreChartVersion, error)
	DeleteChartVersionsByChart(ctx context.Context, chartId int64) error

	// 安装记账
	CreateInstall(ctx context.Context, object *model.AppstoreInstall) (*model.AppstoreInstall, error)
	UpdateInstall(ctx context.Context, id int64, updates map[string]interface{}) error
	GetInstall(ctx context.Context, id int64) (*model.AppstoreInstall, error)
	// GetInstallByTriple 按 (cluster, namespace, release_name) 自然身份读取安装记账；不存在返回 (nil, nil)。
	GetInstallByTriple(ctx context.Context, cluster, namespace, releaseName string) (*model.AppstoreInstall, error)
	ListInstalls(ctx context.Context, opts ...Options) ([]model.AppstoreInstall, error)
	DeleteInstall(ctx context.Context, id int64) error

	// 异步编排：入队 / 抢占 / 待执行列表 / 回写 / 启动恢复
	EnqueueInstall(ctx context.Context, object *model.AppstoreInstall) (*model.AppstoreInstall, error)
	ClaimPendingInstall(ctx context.Context, id int64) (bool, error)
	ListPendingInstalls(ctx context.Context, limit int) ([]model.AppstoreInstall, error)
	UpdateInstallDeploy(ctx context.Context, id int64, updates map[string]interface{}) error
	MarkStaleInstallingFailed(ctx context.Context) (int64, error)
}

type chart struct {
	db *gorm.DB
}

func newChart(db *gorm.DB) ChartInterface {
	return &chart{db}
}

var _ ChartInterface = &chart{}

// ---- Chart ----

func (c *chart) CreateChart(ctx context.Context, object *model.Chart) (*model.Chart, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := c.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (c *chart) UpdateChart(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := c.db.WithContext(ctx).Model(&model.Chart{}).Where("id = ? and resource_version = ?", id, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (c *chart) GetChart(ctx context.Context, id int64) (*model.Chart, error) {
	var object model.Chart
	if err := c.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (c *chart) GetChartByName(ctx context.Context, name string) (*model.Chart, error) {
	var object model.Chart
	if err := c.db.WithContext(ctx).Where("name = ?", name).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (c *chart) ListCharts(ctx context.Context, opts ...Options) ([]model.Chart, error) {
	var objects []model.Chart
	tx := c.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (c *chart) CountCharts(ctx context.Context, opts ...Options) (int64, error) {
	var total int64
	tx := c.db.WithContext(ctx).Model(&model.Chart{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (c *chart) DeleteChart(ctx context.Context, id int64) error {
	f := c.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Chart{})
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

// IncrementInstallCount 累计安装次数 +1（原子自增）。
func (c *chart) IncrementInstallCount(ctx context.Context, chartId int64) error {
	return c.db.WithContext(ctx).Model(&model.Chart{}).Where("id = ?", chartId).
		UpdateColumn("install_count", gorm.Expr("install_count + 1")).Error
}

func (c *chart) ListInstalledChartIds(ctx context.Context, chartIds []int64) ([]int64, error) {
	if len(chartIds) == 0 {
		return nil, nil
	}
	var ids []int64
	if err := c.db.WithContext(ctx).
		Model(&model.AppstoreInstall{}).
		Where("chart_id IN ?", chartIds).
		Distinct().
		Pluck("chart_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// ---- ChartVersion ----

func (c *chart) CreateChartVersion(ctx context.Context, object *model.AppstoreChartVersion) (*model.AppstoreChartVersion, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := c.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (c *chart) GetChartVersion(ctx context.Context, chartId int64, version string) (*model.AppstoreChartVersion, error) {
	var object model.AppstoreChartVersion
	if err := c.db.WithContext(ctx).Where("chart_id = ? and version = ?", chartId, version).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (c *chart) ListChartVersions(ctx context.Context, chartId int64) ([]model.AppstoreChartVersion, error) {
	var objects []model.AppstoreChartVersion
	if err := c.db.WithContext(ctx).Where("chart_id = ?", chartId).Order("id DESC").Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (c *chart) DeleteChartVersionsByChart(ctx context.Context, chartId int64) error {
	return c.db.WithContext(ctx).Where("chart_id = ?", chartId).Delete(&model.AppstoreChartVersion{}).Error
}

// ---- AppstoreInstall ----

func (c *chart) CreateInstall(ctx context.Context, object *model.AppstoreInstall) (*model.AppstoreInstall, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := c.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (c *chart) UpdateInstall(ctx context.Context, id int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	f := c.db.WithContext(ctx).Model(&model.AppstoreInstall{}).Where("id = ?", id).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (c *chart) GetInstall(ctx context.Context, id int64) (*model.AppstoreInstall, error) {
	var object model.AppstoreInstall
	if err := c.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

// GetInstallByTriple 按 (cluster, namespace, release_name) 自然身份读取安装记账。
func (c *chart) GetInstallByTriple(ctx context.Context, cluster, namespace, releaseName string) (*model.AppstoreInstall, error) {
	var object model.AppstoreInstall
	if err := c.db.WithContext(ctx).
		Where("cluster = ? AND namespace = ? AND release_name = ?", cluster, namespace, releaseName).
		First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (c *chart) ListInstalls(ctx context.Context, opts ...Options) ([]model.AppstoreInstall, error) {
	var objects []model.AppstoreInstall
	tx := c.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Order("id DESC").Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (c *chart) DeleteInstall(ctx context.Context, id int64) error {
	f := c.db.WithContext(ctx).Where("id = ?", id).Delete(&model.AppstoreInstall{})
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

// EnqueueInstall 创建一条待执行（pending）安装记录，供异步编排入队使用。
// DeployStatus/PendingOp 为空时补默认值（install + pending）。
func (c *chart) EnqueueInstall(ctx context.Context, object *model.AppstoreInstall) (*model.AppstoreInstall, error) {
	if object.DeployStatus == "" {
		object.DeployStatus = model.InstallDeployPending
	}
	if object.PendingOp == "" {
		object.PendingOp = model.InstallOpInstall
	}
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := c.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

// ClaimPendingInstall 原子抢占一条 pending 记录（pending → installing）。
// 仅当当前 deploy_status='pending' 时才生效，按 RowsAffected 判定是否抢到，
// 防止多 worker / 多副本重复执行同一任务。
func (c *chart) ClaimPendingInstall(ctx context.Context, id int64) (bool, error) {
	f := c.db.WithContext(ctx).Model(&model.AppstoreInstall{}).
		Where("id = ? AND deploy_status = ?", id, model.InstallDeployPending).
		Updates(map[string]interface{}{
			"deploy_status": model.InstallDeployInstalling,
			"gmt_modified":  time.Now(),
		})
	if f.Error != nil {
		return false, f.Error
	}
	return f.RowsAffected == 1, nil
}

// ListPendingInstalls 返回待执行的安装记录（deploy_status='pending'），按 id 升序保证先进先出。
func (c *chart) ListPendingInstalls(ctx context.Context, limit int) ([]model.AppstoreInstall, error) {
	var objects []model.AppstoreInstall
	tx := c.db.WithContext(ctx).Where("deploy_status = ?", model.InstallDeployPending).Order("id ASC")
	if limit > 0 {
		tx = tx.Limit(limit)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

// UpdateInstallDeploy 回写编排相关字段（deploy_status / pending_op / message / status 等）。
func (c *chart) UpdateInstallDeploy(ctx context.Context, id int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	f := c.db.WithContext(ctx).Model(&model.AppstoreInstall{}).Where("id = ?", id).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

// MarkStaleInstallingFailed 启动恢复：把残留的「installing」记录批量置为 failed，返回受影响条数。
// 用于服务进程异常中断后，纠正「卡在执行中」的僵尸记录，使其可通过重试重新入队。
func (c *chart) MarkStaleInstallingFailed(ctx context.Context) (int64, error) {
	f := c.db.WithContext(ctx).Model(&model.AppstoreInstall{}).
		Where("deploy_status = ?", model.InstallDeployInstalling).
		Updates(map[string]interface{}{
			"deploy_status": model.InstallDeployFailed,
			"message":       "服务异常中断，请重试",
			"gmt_modified":  time.Now(),
		})
	if f.Error != nil {
		return 0, f.Error
	}
	return f.RowsAffected, nil
}

// ---- Options ----

// WithCreatedBy 按 created_by 归属用户过滤（安装记账的归属过滤）。
func WithCreatedBy(userId int64) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if userId == 0 {
			return tx
		}
		return tx.Where("created_by = ?", userId)
	}
}

// WithChartId 按 chart_id 过滤版本/安装记录。
func WithChartId(chartId int64) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if chartId == 0 {
			return tx
		}
		return tx.Where("chart_id = ?", chartId)
	}
}

// WithChartCategory 按分类过滤 chart 目录。
func WithChartCategory(category string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if category == "" {
			return tx
		}
		return tx.Where("category = ?", category)
	}
}

// WithReleaseName 按 helm release 名过滤安装记录。
func WithReleaseName(releaseName string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if releaseName == "" {
			return tx
		}
		return tx.Where("release_name = ?", releaseName)
	}
}

// WithInstallCluster 按集群名过滤安装记录。
// 注意：AppstoreInstall 的列名为 cluster，与通用的 WithClusterName（列 cluster_name）不是一回事，勿混用。
func WithInstallCluster(cluster string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if cluster == "" {
			return tx
		}
		return tx.Where("cluster = ?", cluster)
	}
}
