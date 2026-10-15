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

package model

import "github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"

func init() {
	register(&Chart{})
	register(&AppstoreChartVersion{})
	register(&AppstoreInstall{})
}

// Chart 应用商店中的 Chart 目录项（同一 name 对外聚合，版本落在 chart_versions）。
type Chart struct {
	pixiu.Model

	Name        string `gorm:"column:name;type:varchar(128);not null;uniqueIndex:uk_chart_name" json:"name"`
	Category    string `gorm:"column:category;type:varchar(64)" json:"category"`
	Description string `gorm:"column:description;type:text" json:"description"`
	Icon        string `gorm:"column:icon;type:text" json:"icon"`

	// Source 来源：upload（本地上传）| url（从仓库导入）
	Source string `gorm:"column:source;type:varchar(16);default:'upload';not null" json:"source"`
	// RepoURL 从仓库导入时记录的仓库地址
	RepoURL string `gorm:"column:repo_url;type:varchar(512)" json:"repo_url"`

	// LatestVersion 最新版本号（按上传/导入顺序更新，仅用于列表展示）
	LatestVersion string `gorm:"column:latest_version;type:varchar(64)" json:"latest_version"`

	Downloads int64 `gorm:"column:downloads;default:0;not null" json:"downloads"`

	// InstallCount 累计安装次数（安装成功 +1，不随卸载减少）
	InstallCount int64 `gorm:"column:install_count;default:0;not null" json:"install_count"`

	// CreatedBy 上传/导入者用户 id
	CreatedBy     int64  `gorm:"column:created_by;index:idx_chart_created_by" json:"created_by"`
	CreatedByName string `gorm:"column:created_by_name;type:varchar(64)" json:"created_by_name"`
}

func (*Chart) TableName() string {
	return "charts"
}

// AppstoreChartVersion 应用商店 Chart 的单个版本，指向存储中的一个 .tgz 包。
type AppstoreChartVersion struct {
	pixiu.Model

	ChartId     int64  `gorm:"column:chart_id;not null;uniqueIndex:uk_chart_version,priority:1" json:"chart_id"`
	Version     string `gorm:"column:version;type:varchar(64);not null;uniqueIndex:uk_chart_version,priority:2" json:"version"`
	AppVersion  string `gorm:"column:app_version;type:varchar(64)" json:"app_version"`
	Description string `gorm:"column:description;type:text" json:"description"`
	Icon        string `gorm:"column:icon;type:text" json:"icon"`

	// Digest chart 包 sha256（十六进制小写），用于去重
	Digest string `gorm:"column:digest;type:varchar(64);index:idx_chart_version_digest" json:"digest"`
	Size   int64  `gorm:"column:size;default:0;not null" json:"size"`

	// StorageKey 存储后端中的对象键，形如 charts/<name>-<version>.tgz
	StorageKey string `gorm:"column:storage_key;type:varchar(255);not null" json:"storage_key"`
}

func (*AppstoreChartVersion) TableName() string {
	return "chart_versions"
}

// 平台侧部署生命周期（deploy_status）：与 helm release 状态（status）相互独立，
// 前者描述「本次编排任务」的进度，后者描述「集群内 release」的真实状态。
const (
	InstallDeployPending    = "pending"    // 已入队，等待 worker 执行
	InstallDeployInstalling = "installing" // 已被 worker 认领，执行中
	InstallDeploySuccess    = "success"    // 编排成功
	InstallDeployFailed     = "failed"     // 编排失败（可复用同一条记录重试）
)

// 待执行/正在执行的操作类型（pending_op）。
const (
	InstallOpInstall   = "install"
	InstallOpUpgrade   = "upgrade"
	InstallOpRollback  = "rollback"
	InstallOpUninstall = "uninstall"
)

// AppstoreInstall 应用商店的安装记账（真实状态以集群内 helm release 为准）。
type AppstoreInstall struct {
	pixiu.Model

	ChartId   int64  `gorm:"column:chart_id;index:idx_install_chart_id" json:"chart_id"`
	ChartName string `gorm:"column:chart_name;type:varchar(128)" json:"chart_name"`
	Version   string `gorm:"column:version;type:varchar(64)" json:"version"`

	Cluster     string `gorm:"column:cluster;type:varchar(128);not null;uniqueIndex:uk_install_release,priority:1" json:"cluster"`
	Namespace   string `gorm:"column:namespace;type:varchar(128);not null;uniqueIndex:uk_install_release,priority:2" json:"namespace"`
	ReleaseName string `gorm:"column:release_name;type:varchar(128);not null;uniqueIndex:uk_install_release,priority:3" json:"release_name"`

	// Values 本次安装使用的 values（JSON 文本）
	Values string `gorm:"column:values;type:text" json:"values"`
	// Status 最近一次已知状态（running / failed / superseded / uninstalled 等），helm 侧真实状态
	Status string `gorm:"column:status;type:varchar(32)" json:"status"`

	// DeployStatus 平台侧部署生命周期：pending / installing / success / failed
	DeployStatus string `gorm:"column:deploy_status;type:varchar(32);default:'pending';index:idx_install_deploy_status" json:"deploy_status"`
	// PendingOp 待执行（或正在执行）的操作类型：install / upgrade / rollback / uninstall
	PendingOp string `gorm:"column:pending_op;type:varchar(16)" json:"pending_op"`
	// Message 失败原因/结构化文案
	Message string `gorm:"column:message;type:text" json:"message"`
	// TargetRevision 回滚目标 revision（仅 rollback 使用）
	TargetRevision int `gorm:"column:target_revision;type:int;default:0" json:"target_revision"`

	CreatedBy     int64  `gorm:"column:created_by;index:idx_install_created_by" json:"created_by"`
	CreatedByName string `gorm:"column:created_by_name;type:varchar(64)" json:"created_by_name"`
}

func (*AppstoreInstall) TableName() string {
	return "appstore_installs"
}
