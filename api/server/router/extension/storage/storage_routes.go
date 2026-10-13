package storage

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type router struct{ c controller.PixiuInterface }

type meta struct {
	DatasourceID int64 `uri:"datasourceId"`
}

type objectOptions struct {
	Bucket string `form:"bucket" binding:"required"`
	Prefix string `form:"prefix"`
}

type presignOptions struct {
	Bucket string `form:"bucket" binding:"required"`
	Key    string `form:"key" binding:"required"`
	Expiry int    `form:"expiry"`
}

func RegisterStorage(o *options.Options, group *apiregistry.Group) {
	r := &router{c: o.Controller}
	group.Entries = append(group.Entries,
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/ping", Handler: r.pingAdhoc, Description: "对象存储临时连接测试"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/ping", Handler: r.ping, Description: "对象存储连接测试"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/buckets", Handler: r.buckets, Description: "对象存储 Bucket 列表"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/objects", Handler: r.objects, Description: "对象存储对象列表"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/presign-download", Handler: r.presignDownload, Description: "生成对象存储下载地址"},
	)
}

func (r *router) pingAdhoc(c *gin.Context) {
	resp := httputils.NewResponse()
	var cfg types.StorageSourceConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().PingAdhoc(c, &cfg)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) ping(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().Ping(c, m.DatasourceID)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) buckets(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().ListBuckets(c, m.DatasourceID)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) objects(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts objectOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().ListObjects(c, m.DatasourceID, opts.Bucket, opts.Prefix)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) presignDownload(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts presignOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().PresignedGetObject(c, m.DatasourceID, opts.Bucket, opts.Key, opts.Expiry)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = gin.H{"url": result, "expires": opts.Expiry}
	httputils.SetSuccess(c, resp)
}
