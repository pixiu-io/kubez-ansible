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

// Package menu 提供集中式菜单权限目录与解析。
// 菜单码与 API 码分离：menus 控制侧栏/路由可见性，buttons(METHOD:path) 控制接口与按钮。
package menu

// Definition 菜单目录项（配置中心化的代码实现；后续可迁到 DB/配置服务）。
type Definition struct {
	Code         string   `json:"code"`
	ParentCode   string   `json:"parent_code,omitempty"`
	Title        string   `json:"title"`
	Path         string   `json:"path,omitempty"`
	Kind         string   `json:"kind"` // directory | menu
	Public       bool     `json:"public,omitempty"`
	AdminOnly    bool     `json:"admin_only,omitempty"`
	RequiredAPIs []string `json:"required_apis,omitempty"` // 任一命中即可（兼容从 API 推导菜单）
}

const (
	KindDirectory = "directory"
	KindMenu      = "menu"
)

// Catalog 全量菜单目录（父子关系 + 默认 API 映射）。
// Title 与前端侧栏 menus.* / 路由 meta.title 展示名保持一致。
func Catalog() []Definition {
	return []Definition{
		{Code: "dashboard", Title: "仪表盘", Path: "/dashboard", Kind: KindDirectory},
		{Code: "dashboard.console", ParentCode: "dashboard", Title: "工作台", Path: "/dashboard/console", Kind: KindMenu, AdminOnly: true},

		{Code: "container", Title: "容器服务", Path: "/container", Kind: KindDirectory},
		{Code: "container.cluster", ParentCode: "container", Title: "集群", Path: "/container/cluster", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/clusters"}},
		{Code: "container.plan", ParentCode: "container", Title: "部署", Path: "/container/plan", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/plans"}},
		{Code: "container.agent", ParentCode: "container", Title: "Agent", Path: "/container/agent", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/agents"}},

		{Code: "middleware", Title: "中间件", Path: "/middleware", Kind: KindDirectory},
		{Code: "middleware.elasticsearch", ParentCode: "middleware", Title: "Elasticsearch", Path: "/middleware/elasticsearch", Kind: KindMenu, AdminOnly: true},
		{Code: "middleware.redis", ParentCode: "middleware", Title: "Redis", Path: "/middleware/redis", Kind: KindMenu, AdminOnly: true},
		{Code: "middleware.nacos", ParentCode: "middleware", Title: "Nacos", Path: "/middleware/nacos", Kind: KindMenu, AdminOnly: true},
		{Code: "middleware.mysql", ParentCode: "middleware", Title: "MySQL", Path: "/middleware/mysql", Kind: KindMenu, AdminOnly: true},
		{Code: "middleware.rabbitmq", ParentCode: "middleware", Title: "RabbitMQ", Path: "/middleware/rabbitmq", Kind: KindMenu, AdminOnly: true},
		{Code: "middleware.postgres", ParentCode: "middleware", Title: "PostgreSQL", Path: "/middleware/postgres", Kind: KindMenu, AdminOnly: true},

		{Code: "monitor", Title: "监控告警", Path: "/monitor", Kind: KindDirectory},
		{Code: "monitor.realtime", ParentCode: "monitor", Title: "实时查询", Path: "/monitor/realtime-query", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/datasources"}},
		{Code: "monitor.logs", ParentCode: "monitor", Title: "日志", Path: "/monitor/logs", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/datasources"}},
		{Code: "monitor.alert", ParentCode: "monitor", Title: "告警中心", Path: "/monitor/alert-config", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/alerts/rules"}},
		// 数据源独立为顶级菜单，与「监控告警」同级；菜单码保持不变，兼容既有 role_menus 绑定
		{Code: "monitor.datasource", Title: "数据源", Path: "/datasource", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/datasources"}},

		{Code: "ai", Title: "智能助手", Path: "/ai", Kind: KindDirectory},
		{Code: "ai.account", ParentCode: "ai", Title: "AI 账号", Path: "/ai/ai-account", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/assistant/accounts"}},

		{Code: "safeguard", Title: "运维管理", Path: "/safeguard", Kind: KindDirectory},
		{Code: "safeguard.runner", ParentCode: "safeguard", Title: "Runner", Path: "/safeguard/runner", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/runners"}},
		{Code: "safeguard.host", ParentCode: "safeguard", Title: "主机管理", Path: "/safeguard/host", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/nodes"}},

		// 应用资源（应用商店）仅管理员可见：整组 API 已在路由层统一限管理员（普通用户 403），菜单同步 AdminOnly
		{Code: "appstore", Title: "应用商店", Path: "/appstore", Kind: KindMenu, AdminOnly: true},

		{Code: "system", Title: "安全管理", Path: "/system", Kind: KindDirectory},
		{Code: "system.role", ParentCode: "system", Title: "角色", Path: "/system/role", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/roles"}},
		{Code: "system.permission", ParentCode: "system", Title: "授权", Path: "/system/permission", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/clusters/permissions"}},
		{Code: "system.audit", ParentCode: "system", Title: "审计", Path: "/system/audit", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/audits"}},
		{Code: "system.user-center", ParentCode: "system", Title: "个人中心", Path: "/system/user-center", Kind: KindMenu, Public: true},

		{Code: "system-mgr", Title: "系统管理", Path: "/system-mgr", Kind: KindDirectory},
		{Code: "system.user", ParentCode: "system-mgr", Title: "用户", Path: "/system-mgr/user", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/users"}},
		{Code: "system.tenant", ParentCode: "system-mgr", Title: "租户", Path: "/system-mgr/tenant", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/tenants"}},
		{Code: "system.api", ParentCode: "system-mgr", Title: "API", Path: "/system-mgr/api", Kind: KindMenu, RequiredAPIs: []string{"GET:/pixiu/apis"}},
		{Code: "system.email", ParentCode: "system-mgr", Title: "系统邮件", Path: "/system/email", Kind: KindMenu, AdminOnly: true},
	}
}

// ByCode 返回 code -> Definition 映射。
func ByCode() map[string]Definition {
	items := Catalog()
	out := make(map[string]Definition, len(items))
	for _, d := range items {
		out[d.Code] = d
	}
	return out
}

// ValidCodes 返回目录中全部合法菜单码。
func ValidCodes() map[string]struct{} {
	items := Catalog()
	out := make(map[string]struct{}, len(items))
	for _, d := range items {
		out[d.Code] = struct{}{}
	}
	return out
}
