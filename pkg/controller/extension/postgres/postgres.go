package postgres

import (
	"context"
	"database/sql"
	"fmt"
	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	_ "github.com/lib/pq"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Interface interface {
	Ping(context.Context, int64) (*types.PostgresPing, error)
	PingAdhoc(context.Context, int64, *types.PostgresSourceConfig) (*types.PostgresPing, error)
	Info(context.Context, int64) (*types.PostgresServerInfo, error)
	ListDatabases(context.Context, int64) ([]types.PostgresDatabase, error)
	ListSchemas(context.Context, int64, string) ([]types.PostgresSchema, error)
	ListTables(context.Context, int64, string, string) ([]types.PostgresTable, error)
	ExecuteSQL(context.Context, int64, *types.PostgresQueryRequest) (*types.PostgresQueryResult, error)
	ExecuteBatchSQL(context.Context, int64, *types.PostgresBatchRequest) (*types.PostgresBatchResult, error)
	GetTableDetail(context.Context, int64, string, string, string) (*types.PostgresTableDetail, error)
	CreateTable(context.Context, int64, *types.PostgresCreateTableRequest) error
	AlterTable(context.Context, int64, *types.PostgresAlterTableRequest) error
	ListUsers(context.Context, int64) ([]types.PostgresUser, error)
	CreateUser(context.Context, int64, *types.PostgresCreateUserRequest) error
	DeleteUser(context.Context, int64, string) error
	GrantRole(context.Context, int64, *types.PostgresGrantRequest) error
	ListSlowQueries(context.Context, int64, int64, int64, string, string) (*types.PostgresSlowQueryList, error)
	ListSessions(context.Context, int64) ([]types.PostgresSession, error)
	CancelSession(context.Context, int64, int64, bool) error
}
type cached struct {
	db          *sql.DB
	version     int64
	fingerprint string
	last        time.Time
}
type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
	mu      sync.Mutex
	conns   map[string]cached
}

func New(c config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: c, factory: f, conns: map[string]cached{}}
}
func requireAdmin(ctx context.Context) error {
	u, e := httputils.GetUserFromContext(ctx)
	if e != nil {
		return e
	}
	if u.Role != model.RoleRoot && u.Role != model.RoleAdmin {
		return apierrors.ErrForbidden
	}
	return nil
}

// sslModes 返回 sslmode 尝试顺序。lib/pq 不支持 prefer，
// 按“优先 SSL、失败回退明文”语义模拟为“先 require 后 disable”
func sslModes(mode string) []string {
	if mode == "" || mode == "prefer" {
		return []string{"require", "disable"}
	}
	return []string{mode}
}
func dsn(c *types.PostgresSourceConfig, mode string) string {
	port := c.NormalizePort()
	dbn := c.Database
	if dbn == "" {
		dbn = "postgres"
	}
	if mode == "" {
		mode = "require"
	}
	// lib/pq 的 key/value 格式不做 URL 解码，含特殊字符的密码等参数会解析失败，
	// 统一使用 URL 形式连接串，由 lib/pq 按 URL 规则解码
	q := url.Values{}
	q.Set("sslmode", mode)
	if c.ConnectTimeout > 0 {
		q.Set("connect_timeout", strconv.Itoa(c.ConnectTimeout))
	}
	if c.ApplicationName != "" {
		q.Set("application_name", c.ApplicationName)
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.UserName, c.Password),
		Host:     net.JoinHostPort(c.Host, strconv.Itoa(port)),
		Path:     "/" + dbn,
		RawQuery: q.Encode(),
	}
	return u.String()
}
func open(c *types.PostgresSourceConfig) (*sql.DB, error) {
	if strings.TrimSpace(c.Host) == "" || strings.TrimSpace(c.UserName) == "" {
		return nil, apierrors.NewError(fmt.Errorf("postgres host and user_name are required"), http.StatusBadRequest)
	}
	var lastErr error
	for _, mode := range sslModes(c.SSLMode) {
		db, e := sql.Open("postgres", dsn(c, mode))
		if e != nil {
			return nil, e
		}
		// 连接池配置：保持足够的空闲连接，避免频繁创建 TCP 连接
		db.SetMaxIdleConns(10)
		db.SetMaxOpenConns(20)
		db.SetConnMaxLifetime(30 * time.Minute)
		db.SetConnMaxIdleTime(10 * time.Minute)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		e = db.PingContext(ctx)
		cancel()
		if e == nil {
			return db, nil
		}
		db.Close()
		lastErr = e
	}
	return nil, apierrors.NewError(fmt.Errorf("postgres connection failed: %v", lastErr), http.StatusBadGateway)
}
func (c *controller) conn(ctx context.Context, id int64) (*sql.DB, *types.PostgresSourceConfig, error) {
	o, e := c.factory.Datasource().Get(ctx, id)
	if e != nil {
		return nil, nil, apierrors.ErrServerInternal
	}
	if o == nil {
		return nil, nil, apierrors.NewError(fmt.Errorf("datasource not found"), 404)
	}
	if o.Type != model.DatasourceTypeMiddleware || o.SubType != model.DatasourceSubTypePostgres {
		return nil, nil, apierrors.NewError(fmt.Errorf("datasource(%d) is not a postgres datasource", id), 400)
	}
	if !o.External {
		return nil, nil, apierrors.NewError(fmt.Errorf("postgres datasource only supports external direct connection"), 400)
	}
	if e = controllerutil.CheckResourceAccess(ctx, c.factory, o.UserId, types.ResourceTypeDatasource, id); e != nil {
		return nil, nil, e
	}
	var cfg types.DatasourceConfig
	if e = cfg.Unmarshal(o.Config); e != nil {
		return nil, nil, apierrors.ErrServerInternal
	}
	if cfg.Postgres == nil {
		return nil, nil, apierrors.NewError(fmt.Errorf("missing postgres config"), 400)
	}
	p := cfg.Postgres
	key := strconv.FormatInt(id, 10)
	fp := dsn(p, p.SSLMode)
	c.mu.Lock()
	defer c.mu.Unlock()
	if x, ok := c.conns[key]; ok {
		// 配置未变化时复用缓存连接；缓存连接可能已失效，复用前先做健康检查
		if x.version == o.ResourceVersion && x.fingerprint == fp {
			pingCtx, pingCancel := context.WithTimeout(ctx, 3*time.Second)
			pingErr := x.db.PingContext(pingCtx)
			pingCancel()
			if pingErr == nil {
				x.last = time.Now()
				return x.db, p, nil
			}
		}
		x.db.Close()
		delete(c.conns, key)
	}
	db, e := open(p)
	if e != nil {
		return nil, nil, e
	}
	c.conns[key] = cached{db: db, version: o.ResourceVersion, fingerprint: fp, last: time.Now()}
	return db, p, nil
}
func (c *controller) Ping(ctx context.Context, id int64) (*types.PostgresPing, error) {
	db, p, e := c.conn(ctx, id)
	if e != nil {
		return nil, e
	}
	return ping(ctx, db, p), nil
}

// PingAdhoc 使用当前表单配置做临时探测；id > 0 时读取已保存的数据源，
// 并在密码为空时补齐已保存密码。
func (c *controller) PingAdhoc(ctx context.Context, id int64, p *types.PostgresSourceConfig) (*types.PostgresPing, error) {
	if e := requireAdmin(ctx); e != nil {
		return nil, e
	}

	if id > 0 {
		object, e := c.factory.Datasource().Get(ctx, id)
		if e != nil || object == nil {
			if e != nil {
				return nil, apierrors.ErrServerInternal
			}
			return nil, apierrors.NewError(fmt.Errorf("datasource not found"), http.StatusNotFound)
		}
		if e = controllerutil.CheckResourceAccess(ctx, c.factory, object.UserId, types.ResourceTypeDatasource, id); e != nil {
			return nil, e
		}
		var saved types.DatasourceConfig
		if e = saved.Unmarshal(object.Config); e != nil || saved.Postgres == nil {
			return nil, apierrors.NewError(fmt.Errorf("invalid PostgreSQL datasource config"), http.StatusBadRequest)
		}
		if p == nil {
			p = &types.PostgresSourceConfig{}
		}
		if p.Password == "" {
			p.Password = saved.Postgres.Password
		}
	}

	db, e := open(p)
	if e != nil {
		return nil, e
	}
	defer db.Close()
	return ping(ctx, db, p), nil
}

func ping(ctx context.Context, db *sql.DB, p *types.PostgresSourceConfig) *types.PostgresPing {
	r := &types.PostgresPing{Address: p.DisplayAddress()}
	st := time.Now()
	if e := db.PingContext(ctx); e != nil {
		r.Message = e.Error()
		return r
	}
	r.Connected = true
	r.LatencyMs = time.Since(st).Milliseconds()
	// 与 MySQL 的短版本串保持一致，取 server_version 而非完整 version() 描述
	_ = db.QueryRowContext(ctx, "select current_setting('server_version')").Scan(&r.Version)
	return r
}

// pgScanOne 执行单值查询并忽略错误（概览指标允许部分缺失）
func pgScanOne(ctx context.Context, db *sql.DB, query string, dest ...any) {
	_ = db.QueryRowContext(ctx, query).Scan(dest...)
}

// pgQuerySlice 执行查询并将每行扫描为 T，收敛 List* 的 rows.Next/Scan 重复模板
func pgQuerySlice[T any](ctx context.Context, db *sql.DB, query string, scan func(*sql.Rows) (T, error), args ...any) ([]T, error) {
	rows, e := db.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		x, e := scan(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (c *controller) Info(ctx context.Context, id int64) (*types.PostgresServerInfo, error) {
	db, _, e := c.conn(ctx, id)
	if e != nil {
		return nil, e
	}
	r := &types.PostgresServerInfo{}
	pgScanOne(ctx, db, "select current_setting('server_version')", &r.Version)
	pgScanOne(ctx, db, "select extract(epoch from now()-pg_postmaster_start_time())::bigint", &r.UptimeSeconds)
	pgScanOne(ctx, db, "select count(*) from pg_stat_activity", &r.Connections)
	pgScanOne(ctx, db, "select setting::bigint from pg_settings where name='max_connections'", &r.MaxConnections)
	pgScanOne(ctx, db, "select count(*) filter (where state='active'),count(*) filter (where state='idle'),count(*) filter (where wait_event is not null),count(*) filter (where state='idle in transaction') from pg_stat_activity", &r.ActiveSessions, &r.IdleSessions, &r.WaitingSessions, &r.IdleInTxSessions)
	pgScanOne(ctx, db, "select count(*) filter (where datallowconn),count(*) filter (where not datistemplate) from pg_database", &r.DatabaseCount, &r.UserDatabaseCount)
	pgScanOne(ctx, db, "select pg_is_in_recovery()", &r.InRecovery)
	pgScanOne(ctx, db, "select coalesce(sum(pg_database_size(datname)),0) from pg_database where datallowconn", &r.DataSizeBytes)
	var hit, read int64
	pgScanOne(ctx, db, "select coalesce(sum(blks_hit),0),coalesce(sum(blks_read),0),coalesce(sum(xact_commit),0)+coalesce(sum(xact_rollback),0),coalesce(sum(temp_files),0),coalesce(sum(temp_bytes),0),coalesce(sum(deadlocks),0) from pg_stat_database", &hit, &read, &r.TotalTransactions, &r.TempFiles, &r.TempBytes, &r.Deadlocks)
	if hit+read > 0 {
		r.CacheHitRatio = float64(hit) / float64(hit+read) * 100
	}
	if r.UptimeSeconds > 0 {
		r.TPS = math.Round(float64(r.TotalTransactions)/float64(r.UptimeSeconds)*10) / 10
	}
	pgScanOne(ctx, db, "select count(*) from pg_locks where not granted", &r.BlockedSessions)
	pgScanOne(ctx, db, "select count(distinct kl.pid) from pg_locks bl join pg_locks kl on kl.locktype=bl.locktype and kl.database is not distinct from bl.database and kl.relation is not distinct from bl.relation and kl.page is not distinct from bl.page and kl.tuple is not distinct from bl.tuple and kl.virtualxid is not distinct from bl.virtualxid and kl.transactionid is not distinct from bl.transactionid and kl.classid is not distinct from bl.classid and kl.objid is not distinct from bl.objid and kl.objsubid is not distinct from bl.objsubid and kl.pid<>bl.pid where not bl.granted and kl.granted", &r.BlockingSessions)
	pgScanOne(ctx, db, "select pg_size_bytes(current_setting('shared_buffers'))", &r.SharedBuffersBytes)
	pgScanOne(ctx, db, "select pg_size_bytes(current_setting('effective_cache_size'))", &r.EffectiveCacheBytes)
	var hasExt bool
	if db.QueryRowContext(ctx, "select exists(select 1 from pg_extension where extname='pg_stat_statements')").Scan(&hasExt) == nil && hasExt {
		r.PgStatStatements = true
		// PG13+ 列名为 mean_exec_time，PG12- 为 mean_time，列不存在时回退另一种
		if e := db.QueryRowContext(ctx, "select count(*) from pg_stat_statements where mean_exec_time>1000").Scan(&r.SlowQueries); e != nil {
			pgScanOne(ctx, db, "select count(*) from pg_stat_statements where mean_time>1000", &r.SlowQueries)
		}
	}
	// version 为空说明关键查询失败（连接可能已失效），返回错误让前端重试
	if r.Version == "" {
		return nil, apierrors.NewError(fmt.Errorf("postgres instance info query failed, connection may be stale"), http.StatusBadGateway)
	}
	return r, nil
}
func (c *controller) ListDatabases(ctx context.Context, id int64) ([]types.PostgresDatabase, error) {
	db, _, e := c.conn(ctx, id)
	if e != nil {
		return nil, e
	}
	return pgQuerySlice(ctx, db, "select datname,pg_get_userbyid(datdba),pg_database_size(datname),pg_encoding_to_char(encoding),datcollate from pg_database where datallowconn order by datname", func(r *sql.Rows) (types.PostgresDatabase, error) {
		var x types.PostgresDatabase
		e := r.Scan(&x.Name, &x.Owner, &x.SizeBytes, &x.Encoding, &x.Collation)
		return x, e
	})
}
func (c *controller) ListSchemas(ctx context.Context, id int64, dbn string) ([]types.PostgresSchema, error) {
	db, _, e := c.conn(ctx, id)
	if e != nil {
		return nil, e
	}
	return pgQuerySlice(ctx, db, "select schema_name,coalesce(schema_owner,'') from information_schema.schemata where schema_name not like 'pg_%' and schema_name <> 'information_schema' order by schema_name", func(r *sql.Rows) (types.PostgresSchema, error) {
		var x types.PostgresSchema
		e := r.Scan(&x.Name, &x.Owner)
		return x, e
	})
}
func (c *controller) ListTables(ctx context.Context, id int64, dbn, schema string) ([]types.PostgresTable, error) {
	db, _, e := c.conn(ctx, id)
	if e != nil {
		return nil, e
	}
	return pgQuerySlice(ctx, db, "select n.nspname,c.relname,case c.relkind when 'r' then 'table' when 'v' then 'view' when 'm' then 'materialized_view' when 'S' then 'sequence' else c.relkind::text end,coalesce(c.reltuples,0)::bigint,pg_total_relation_size(c.oid) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname=$1 and c.relkind in ('r','v','m','S') order by c.relname", func(r *sql.Rows) (types.PostgresTable, error) {
		var x types.PostgresTable
		e := r.Scan(&x.Schema, &x.Name, &x.Type, &x.Rows, &x.SizeBytes)
		return x, e
	}, schema)
}
func (c *controller) ListSessions(ctx context.Context, id int64) ([]types.PostgresSession, error) {
	db, _, e := c.conn(ctx, id)
	if e != nil {
		return nil, e
	}
	return pgQuerySlice(ctx, db, "select pid,coalesce(usename,''),coalesce(client_addr::text,''),coalesce(datname,''),coalesce(application_name,''),coalesce(state,''),coalesce(wait_event,''),coalesce(query_start::text,''),coalesce(extract(epoch from now()-query_start),0),coalesce(query,'') from pg_stat_activity where pid<>pg_backend_pid() order by query_start desc nulls last", func(r *sql.Rows) (types.PostgresSession, error) {
		var x types.PostgresSession
		e := r.Scan(&x.PID, &x.User, &x.ClientAddr, &x.Database, &x.Application, &x.State, &x.WaitEvent, &x.QueryStart, &x.DurationSeconds, &x.Query)
		return x, e
	})
}
func (c *controller) CancelSession(ctx context.Context, id, pid int64, terminate bool) error {
	if e := requireAdmin(ctx); e != nil {
		return e
	}
	db, _, e := c.conn(ctx, id)
	if e != nil {
		return e
	}
	fn := "pg_cancel_backend"
	if terminate {
		fn = "pg_terminate_backend"
	}
	var ok bool
	e = db.QueryRowContext(ctx, "select "+fn+"($1)", pid).Scan(&ok)
	return e
}
