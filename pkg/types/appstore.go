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

package types

// ChartIdMeta chart 相关 URI 参数。
type ChartIdMeta struct {
	Id int64 `uri:"id" binding:"required"`
}

// ReleaseNamespaceMeta release 集群/命名空间定位（URI 参数）。
// 「全部命名空间」路由无 namespace 段，故 Namespace 不加 required，可为空。
type ReleaseNamespaceMeta struct {
	Cluster   string `uri:"cluster" binding:"required"`
	Namespace string `uri:"namespace"`
}

// ReleasePathMeta release 集群/命名空间/名称三元组定位（URI 参数）。
type ReleasePathMeta struct {
	Cluster   string `uri:"cluster" binding:"required"`
	Namespace string `uri:"namespace" binding:"required"`
	Name      string `uri:"name" binding:"required"`
}

// ImportChartRequest 从 Helm 仓库导入 chart 的请求。
type ImportChartRequest struct {
	RepoUrl  string `json:"repo_url" binding:"required"`
	Chart    string `json:"chart" binding:"required"`
	Version  string `json:"version" binding:"required"`
	Username string `json:"username"`
	Password string `json:"password"`
	Category string `json:"category"`
}

// ListChartOption chart 列表查询条件。
type ListChartOption struct {
	Name     string `form:"name" json:"name"`
	Category string `form:"category" json:"category"`
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"pageSize" json:"pageSize"`
}

// ChartVersionInfo chart 版本信息。
type ChartVersionInfo struct {
	Version     string `json:"version"`
	AppVersion  string `json:"app_version"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Digest      string `json:"digest"`
	Size        int64  `json:"size"`
	GmtCreate   string `json:"gmt_create"`
}

// ChartTemplateInfo chart 内的一个模板文件（原始、未渲染）。
type ChartTemplateInfo struct {
	// Name 模板文件在 chart 内的路径，如 templates/alertmanager/secret.yaml
	Name string `json:"name"`
	// Kind 尽力从模板原文解析的资源类型（含 go template 时可能为空/字面值）
	Kind string `json:"kind"`
	// Size 模板原文字节数
	Size int64 `json:"size"`
}

// ChartInfo chart 目录项（含版本列表时用于详情）。
type ChartInfo struct {
	Id            int64  `json:"id"`
	Name          string `json:"name"`
	Category      string `json:"category"`
	Description   string `json:"description"`
	Icon          string `json:"icon"`
	Source        string `json:"source"`
	RepoUrl       string `json:"repo_url"`
	LatestVersion string `json:"latest_version"`
	Downloads     int64  `json:"downloads"`
	// InstallCount 累计安装次数（安装成功 +1，不随卸载减少）
	InstallCount  int64  `json:"install_count"`
	CreatedBy     int64  `json:"created_by"`
	CreatedByName string `json:"created_by_name"`
	// Installed 该 chart 是否至少存在一条安装记录（列表页「已安装」徽标）。
	Installed   bool               `json:"installed"`
	GmtCreate   string             `json:"gmt_create"`
	GmtModified string             `json:"gmt_modified"`
	Versions    []ChartVersionInfo `json:"versions,omitempty"`
}

// ChartListResponse chart 分页列表返回。
type ChartListResponse struct {
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Items    []ChartInfo `json:"items"`
}

// InstallReleaseRequest 安装 release 请求（集群/命名空间由 URI 定位）。
type InstallReleaseRequest struct {
	ChartId     int64                  `json:"chart_id" binding:"required"`
	ReleaseName string                 `json:"release_name" binding:"required"`
	Version     string                 `json:"version" binding:"required"`
	Values      map[string]interface{} `json:"values"`
	// Preview 为 true 时走 helm dry-run（不真正安装），用于安装前预览。
	Preview bool `json:"preview"`
}

// UpgradeChartRequest 升级已安装 release 请求。
type UpgradeChartRequest struct {
	Version string                 `json:"version" binding:"required"`
	Values  map[string]interface{} `json:"values"`
}

// RollbackChartRequest 回滚已安装 release 请求。
type RollbackChartRequest struct {
	Version int `json:"version" binding:"required"`
}

// InstallInfo 已安装 release 信息。
type InstallInfo struct {
	Id          int64  `json:"id"`
	ChartId     int64  `json:"chart_id"`
	ChartName   string `json:"chart_name"`
	Version     string `json:"version"`
	Cluster     string `json:"cluster"`
	Namespace   string `json:"namespace"`
	ReleaseName string `json:"release_name"`
	Values      string `json:"values"`
	// Status 集群内 helm release 的真实状态（running / deployed / failed 等）
	Status string `json:"status"`
	// DeployStatus 平台侧部署生命周期：pending / installing / success / failed
	DeployStatus string `json:"deploy_status"`
	// PendingOp 待执行/正在执行的操作类型：install / upgrade / rollback / uninstall
	PendingOp string `json:"pending_op"`
	// Message 失败原因/结构化文案
	Message string `json:"message"`
	// Managed 是否平台记账（appstore 安装的 release 为 true；集群内外部 release 只读，为 false）
	Managed bool `json:"managed"`
	// TargetRevision 回滚目标 revision
	TargetRevision int    `json:"target_revision"`
	CreatedBy      int64  `json:"created_by"`
	CreatedByName  string `json:"created_by_name"`
	GmtCreate      string `json:"gmt_create"`
	GmtModified    string `json:"gmt_modified"`
}

// InstallHistoryItem release 历史版本项。
type InstallHistoryItem struct {
	Revision int `json:"revision"`
	// Version 与 Revision 同值（helm revision）；前端「历史→回滚」直接回传该值到
	// RollbackChartRequest.version，保证链路口径统一。
	Version     int    `json:"version"`
	Status      string `json:"status"`
	Chart       string `json:"chart"`
	AppVersion  string `json:"app_version"`
	Description string `json:"description"`
	Updated     string `json:"updated"`
}
