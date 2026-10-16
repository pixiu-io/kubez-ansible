package postgres

import (
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/gin-gonic/gin"
)

type router struct{ c controller.PixiuInterface }

type meta struct {
	DatasourceId int64 `uri:"datasourceId"`
}

type opts struct {
	Database string `form:"database"`
	Schema   string `form:"schema"`
}

type sessionOpts struct {
	PID       int64 `form:"pid" binding:"required"`
	Terminate bool  `form:"terminate"`
}

type tableDetailOpts struct {
	Database string `form:"database"`
	Schema   string `form:"schema"`
	Table    string `form:"table"`
}

type userOpts struct {
	Name string `form:"name"`
}

type slowQueryOpts struct {
	Page     int64  `form:"page"`
	PageSize int64  `form:"pageSize"`
	OrderBy  string `form:"orderBy"`
	OrderDir string `form:"orderDir"`
}

type pingPostgresRequest struct {
	DatasourceId int64 `json:"datasource_id"`
	types.PostgresSourceConfig
}

func RegisterPostgres(o *options.Options, g *apiregistry.Group) {
	r := &router{c: o.Controller}
	g.Entries = append(g.Entries,
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/postgres/ping", Handler: r.pingAdhoc, Description: "PostgreSQL 临时连通性探测"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/postgres/:datasourceId/ping", Handler: r.ping, Description: "PostgreSQL 连接探测"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/postgres/:datasourceId/info", Handler: r.info, Description: "PostgreSQL 实例概览"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/postgres/:datasourceId/databases", Handler: r.databases, Description: "PostgreSQL 数据库列表"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/postgres/:datasourceId/schemas", Handler: r.schemas, Description: "PostgreSQL Schema 列表"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/postgres/:datasourceId/tables", Handler: r.tables, Description: "PostgreSQL 表列表"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/postgres/:datasourceId/query", Handler: r.query, Description: "PostgreSQL SQL 控制台"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/postgres/:datasourceId/batch", Handler: r.batchQuery, Description: "PostgreSQL SQL 批量执行"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/postgres/:datasourceId/sessions", Handler: r.sessions, Description: "PostgreSQL 会话列表"},
		apiregistry.RouteEntry{Method: "DELETE", RelativePath: "/postgres/:datasourceId/sessions", Handler: r.cancel, Description: "PostgreSQL 取消或终止会话"},
		// 表管理
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/postgres/:datasourceId/table-detail", Handler: r.tableDetail, Description: "PostgreSQL 表详情"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/postgres/:datasourceId/create-table", Handler: r.createTable, Description: "PostgreSQL 建表"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/postgres/:datasourceId/alter-table", Handler: r.alterTable, Description: "PostgreSQL 编辑表"},
		// 用户管理
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/postgres/:datasourceId/users", Handler: r.users, Description: "PostgreSQL 用户列表"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/postgres/:datasourceId/users", Handler: r.createUser, Description: "PostgreSQL 创建用户"},
		apiregistry.RouteEntry{Method: "DELETE", RelativePath: "/postgres/:datasourceId/users", Handler: r.deleteUser, Description: "PostgreSQL 删除用户"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/postgres/:datasourceId/grant", Handler: r.grantRole, Description: "PostgreSQL 用户授权"},
		// 慢查询
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/postgres/:datasourceId/slow-queries", Handler: r.slowQueries, Description: "PostgreSQL 慢查询分析"})
}

func (r *router) pingAdhoc(c *gin.Context) {
	var q pingPostgresRequest
	res := httputils.NewResponse()
	if e := c.ShouldBindJSON(&q); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	result, e := r.c.Extension().Postgres().PingAdhoc(c, q.DatasourceId, &q.PostgresSourceConfig)
	if e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	res.Result = result
	httputils.SetSuccess(c, res)
}

func (r *router) ping(c *gin.Context) {
	var m meta
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, nil, &m, nil); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	result, e := r.c.Extension().Postgres().Ping(c, m.DatasourceId)
	if e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	res.Result = result
	httputils.SetSuccess(c, res)
}

func (r *router) info(c *gin.Context) {
	var m meta
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, nil, &m, nil); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	result, e := r.c.Extension().Postgres().Info(c, m.DatasourceId)
	if e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	res.Result = result
	httputils.SetSuccess(c, res)
}

func (r *router) databases(c *gin.Context) {
	var m meta
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, nil, &m, nil); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	result, e := r.c.Extension().Postgres().ListDatabases(c, m.DatasourceId)
	if e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	res.Result = result
	httputils.SetSuccess(c, res)
}

func (r *router) schemas(c *gin.Context) {
	var m meta
	var o opts
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, nil, &m, &o); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	result, e := r.c.Extension().Postgres().ListSchemas(c, m.DatasourceId, o.Database)
	if e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	res.Result = result
	httputils.SetSuccess(c, res)
}

func (r *router) tables(c *gin.Context) {
	var m meta
	var o opts
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, nil, &m, &o); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	result, e := r.c.Extension().Postgres().ListTables(c, m.DatasourceId, o.Database, o.Schema)
	if e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	res.Result = result
	httputils.SetSuccess(c, res)
}

func (r *router) query(c *gin.Context) {
	var m meta
	var q types.PostgresQueryRequest
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, &q, &m, nil); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	result, e := r.c.Extension().Postgres().ExecuteSQL(c, m.DatasourceId, &q)
	if e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	res.Result = result
	httputils.SetSuccess(c, res)
}

func (r *router) sessions(c *gin.Context) {
	var m meta
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, nil, &m, nil); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	result, e := r.c.Extension().Postgres().ListSessions(c, m.DatasourceId)
	if e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	res.Result = result
	httputils.SetSuccess(c, res)
}

func (r *router) cancel(c *gin.Context) {
	var m meta
	var o sessionOpts
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, nil, &m, &o); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	if e := r.c.Extension().Postgres().CancelSession(c, m.DatasourceId, o.PID, o.Terminate); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	httputils.SetSuccess(c, res)
}

// ── 批量执行 ──────────────────────────────────────────────────

func (r *router) batchQuery(c *gin.Context) {
	var m meta
	var q types.PostgresBatchRequest
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, &q, &m, nil); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	result, e := r.c.Extension().Postgres().ExecuteBatchSQL(c, m.DatasourceId, &q)
	if e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	res.Result = result
	httputils.SetSuccess(c, res)
}

// ── 表管理 ────────────────────────────────────────────────────

func (r *router) tableDetail(c *gin.Context) {
	var m meta
	var o tableDetailOpts
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, nil, &m, &o); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	result, e := r.c.Extension().Postgres().GetTableDetail(c, m.DatasourceId, o.Database, o.Schema, o.Table)
	if e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	res.Result = result
	httputils.SetSuccess(c, res)
}

func (r *router) createTable(c *gin.Context) {
	var m meta
	var q types.PostgresCreateTableRequest
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, &q, &m, nil); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	if e := r.c.Extension().Postgres().CreateTable(c, m.DatasourceId, &q); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	httputils.SetSuccess(c, res)
}

func (r *router) alterTable(c *gin.Context) {
	var m meta
	var q types.PostgresAlterTableRequest
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, &q, &m, nil); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	if e := r.c.Extension().Postgres().AlterTable(c, m.DatasourceId, &q); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	httputils.SetSuccess(c, res)
}

// ── 用户管理 ──────────────────────────────────────────────────

func (r *router) users(c *gin.Context) {
	var m meta
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, nil, &m, nil); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	result, e := r.c.Extension().Postgres().ListUsers(c, m.DatasourceId)
	if e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	res.Result = result
	httputils.SetSuccess(c, res)
}

func (r *router) createUser(c *gin.Context) {
	var m meta
	var q types.PostgresCreateUserRequest
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, &q, &m, nil); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	if e := r.c.Extension().Postgres().CreateUser(c, m.DatasourceId, &q); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	httputils.SetSuccess(c, res)
}

func (r *router) deleteUser(c *gin.Context) {
	var m meta
	var o userOpts
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, nil, &m, &o); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	if e := r.c.Extension().Postgres().DeleteUser(c, m.DatasourceId, o.Name); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	httputils.SetSuccess(c, res)
}

func (r *router) grantRole(c *gin.Context) {
	var m meta
	var q types.PostgresGrantRequest
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, &q, &m, nil); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	if e := r.c.Extension().Postgres().GrantRole(c, m.DatasourceId, &q); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	httputils.SetSuccess(c, res)
}

// ── 慢查询 ────────────────────────────────────────────────────

func (r *router) slowQueries(c *gin.Context) {
	var m meta
	var o slowQueryOpts
	res := httputils.NewResponse()
	if e := httputils.ShouldBindAny(c, nil, &m, &o); e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	result, e := r.c.Extension().Postgres().ListSlowQueries(c, m.DatasourceId, o.Page, o.PageSize, o.OrderBy, o.OrderDir)
	if e != nil {
		httputils.SetFailed(c, res, e)
		return
	}
	res.Result = result
	httputils.SetSuccess(c, res)
}
