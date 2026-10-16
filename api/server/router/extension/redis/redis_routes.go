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

package redis

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type meta struct {
	DatasourceId int64 `uri:"datasourceId"`
}

type scanOptions struct {
	Cursor uint64 `form:"cursor"`
	Match  string `form:"match"`
	Count  int64  `form:"count"`
	DB     *int   `form:"db"`
}

type keyOptions struct {
	Key string `form:"key" binding:"required"`
	DB  *int   `form:"db"`
}

type dbOptions struct {
	DB *int `form:"db"`
}

type pingRedisRequest struct {
	DatasourceId int64 `json:"datasource_id"`
	types.RedisSourceConfig
}

// pingRedisAdhoc 临时探测：请求体直接传连接配置，不依赖已保存的数据源
func (rr *redisRouter) pingRedisAdhoc(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		req pingRedisRequest
		err error
	)
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = rr.c.Extension().Redis().PingAdhoc(c, req.DatasourceId, &req.RedisSourceConfig); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (rr *redisRouter) pingRedis(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = rr.c.Extension().Redis().Ping(c, m.DatasourceId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (rr *redisRouter) getRedisInfo(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m    meta
		opts dbOptions
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = rr.c.Extension().Redis().Info(c, m.DatasourceId, opts.DB); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (rr *redisRouter) scanRedisKeys(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m    meta
		opts scanOptions
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = rr.c.Extension().Redis().ScanKeys(c, m.DatasourceId, opts.DB, opts.Cursor, opts.Match, opts.Count); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (rr *redisRouter) getRedisKeyDetail(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m    meta
		opts keyOptions
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = rr.c.Extension().Redis().GetKeyDetail(c, m.DatasourceId, opts.DB, opts.Key); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// createRedisKey 新增 key（写操作，第一版仅 string）
func (rr *redisRouter) createRedisKey(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		req types.RedisCreateKeyRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = rr.c.Extension().Redis().CreateKey(c, m.DatasourceId, req.DB, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// deleteRedisKey 删除 key（写操作）
func (rr *redisRouter) deleteRedisKey(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m    meta
		opts keyOptions
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = rr.c.Extension().Redis().DeleteKey(c, m.DatasourceId, opts.DB, opts.Key); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// deleteRedisKeys 批量删除 key（写操作），请求体传 keys 数组，返回实际删除数量
func (rr *redisRouter) deleteRedisKeys(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		req types.RedisDeleteKeysRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = rr.c.Extension().Redis().DeleteKeys(c, m.DatasourceId, req.DB, req.Keys); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// updateRedisKeyValue 修改 string 类型 key 的值（写操作，保持原 TTL）
func (rr *redisRouter) updateRedisKeyValue(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		req types.RedisUpdateKeyValueRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = rr.c.Extension().Redis().UpdateKeyValue(c, m.DatasourceId, req.DB, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// setRedisKeyTTL 修改 key TTL（写操作）
func (rr *redisRouter) setRedisKeyTTL(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		req types.RedisSetTTLRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = rr.c.Extension().Redis().SetKeyTTL(c, m.DatasourceId, req.DB, req.Key, req.TTL); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
