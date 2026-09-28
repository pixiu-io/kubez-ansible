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

package storage

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

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

type bucketMeta struct {
	DatasourceID int64  `uri:"datasourceId"`
	Bucket       string `uri:"bucket" binding:"required"`
}

type createBucketOptions struct {
	Name string `json:"name" binding:"required"`
}

type objectPageOptions struct {
	Bucket    string `form:"bucket" binding:"required"`
	Prefix    string `form:"prefix"`
	Token     string `form:"token"`
	PageSize  int    `form:"page_size"`
	Keyword   string `form:"keyword"`
	Recursive bool   `form:"recursive"`
}

type uploadOptions struct {
	Bucket string `form:"bucket" binding:"required"`
	Key    string `form:"key" binding:"required"`
}

type deleteObjectsOptions struct {
	Bucket string   `json:"bucket" binding:"required"`
	Keys   []string `json:"keys" binding:"required"`
}

type renameObjectOptions struct {
	Bucket string `json:"bucket" binding:"required"`
	Src    string `json:"src" binding:"required"`
	Dst    string `json:"dst" binding:"required"`
}

type presignOptions struct {
	Bucket string `form:"bucket" binding:"required"`
	Key    string `form:"key" binding:"required"`
	Expiry int    `form:"expiry"`
}

type userMeta struct {
	DatasourceID int64  `uri:"datasourceId"`
	AccessKey    string `uri:"accessKey" binding:"required"`
}

type versionListOptions struct {
	Bucket  string `form:"bucket" binding:"required"`
	Prefix  string `form:"prefix"`
	Recycle bool   `form:"recycle"`
	Limit   int    `form:"limit"`
}

type bucketOnlyOptions struct {
	Bucket string `form:"bucket" binding:"required"`
}

type versionTargetOptions struct {
	Bucket    string `json:"bucket" binding:"required"`
	Key       string `json:"key" binding:"required"`
	VersionID string `json:"version_id" binding:"required"`
}

type incompleteUploadOptions struct {
	Bucket string `form:"bucket" binding:"required"`
	Prefix string `form:"prefix"`
}

type abortUploadOptions struct {
	Bucket string   `json:"bucket" binding:"required"`
	Keys   []string `json:"keys" binding:"required"`
}

type objectTagsOptions struct {
	Bucket    string `form:"bucket" binding:"required"`
	Key       string `form:"key" binding:"required"`
	VersionID string `form:"version_id"`
}

type objectTagsUpdateOptions struct {
	Bucket    string                   `json:"bucket" binding:"required"`
	Key       string                   `json:"key" binding:"required"`
	VersionID string                   `json:"version_id"`
	Tags      []types.StorageObjectTag `json:"tags"`
}

type objectDetailOptions struct {
	Bucket    string `form:"bucket" binding:"required"`
	Key       string `form:"key" binding:"required"`
	VersionID string `form:"version_id"`
}

type objectMetaUpdateOptions struct {
	Bucket string                        `json:"bucket" binding:"required"`
	Key    string                        `json:"key" binding:"required"`
	Meta   types.StorageObjectMetaUpdate `json:"meta"`
}

// multipartPartOptions 分片上传：分片本体走请求体，其余信息走 query，避免 multipart/form-data 开销
type multipartPartOptions struct {
	Bucket     string `form:"bucket" binding:"required"`
	Key        string `form:"key" binding:"required"`
	UploadID   string `form:"upload_id" binding:"required"`
	PartNumber int    `form:"part_number" binding:"required"`
}

type multipartPartsOptions struct {
	Bucket   string `form:"bucket" binding:"required"`
	Key      string `form:"key" binding:"required"`
	UploadID string `form:"upload_id" binding:"required"`
}

type bucketPolicyOptions struct {
	Policy string `json:"policy"`
}

type transferObjectsOptions struct {
	Bucket    string   `json:"bucket" binding:"required"`
	Keys      []string `json:"keys" binding:"required"`
	DstBucket string   `json:"dst_bucket" binding:"required"`
	DstPrefix string   `json:"dst_prefix"`
	Move      bool     `json:"move"`
}

// batchDownloadOptions 批量/目录打包下载：keys 用重复参数传递（keys=a&keys=b）
type batchDownloadOptions struct {
	Bucket string   `form:"bucket" binding:"required"`
	Keys   []string `form:"keys" binding:"required"`
}

func RegisterStorage(o *options.Options, group *apiregistry.Group) {
	r := &router{c: o.Controller}
	group.Entries = append(group.Entries,
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/ping", Handler: r.pingAdhoc, Description: "对象存储临时连接测试"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/ping", Handler: r.ping, Description: "对象存储连接测试"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/buckets", Handler: r.buckets, Description: "对象存储 Bucket 列表"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/buckets", Handler: r.createBucket, Description: "创建对象存储 Bucket"},
		apiregistry.RouteEntry{Method: "DELETE", RelativePath: "/storage/:datasourceId/buckets/:bucket", Handler: r.deleteBucket, Description: "删除对象存储 Bucket"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/buckets/:bucket/config", Handler: r.getBucketConfig, Description: "获取对象存储 Bucket 配置"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/buckets/:bucket/config", Handler: r.setBucketConfig, Description: "设置对象存储 Bucket 配置"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/buckets/:bucket/empty", Handler: r.emptyBucket, Description: "清空对象存储 Bucket"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/buckets/:bucket/lifecycle", Handler: r.getLifecycle, Description: "获取对象存储 Bucket 生命周期规则"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/buckets/:bucket/lifecycle", Handler: r.setLifecycle, Description: "设置对象存储 Bucket 生命周期规则"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/buckets/:bucket/cors", Handler: r.getCors, Description: "获取对象存储 Bucket 跨域规则"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/buckets/:bucket/cors", Handler: r.setCors, Description: "设置对象存储 Bucket 跨域规则"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/objects", Handler: r.objects, Description: "对象存储对象列表"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/objects", Handler: r.uploadObject, Description: "上传对象存储对象"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/objects/delete", Handler: r.deleteObjects, Description: "删除对象存储对象"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/objects/rename", Handler: r.renameObject, Description: "重命名对象存储对象"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/presign-download", Handler: r.presignDownload, Description: "生成对象存储下载地址"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/object-versions", Handler: r.objectVersions, Description: "对象存储历史版本/回收站列表"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/bucket-versioning", Handler: r.bucketVersioning, Description: "查询对象存储 Bucket 版本控制状态"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/object-versions/restore", Handler: r.restoreObjectVersion, Description: "恢复对象存储历史版本"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/object-versions/delete", Handler: r.deleteObjectVersion, Description: "彻底删除对象存储历史版本"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/incomplete-uploads", Handler: r.incompleteUploads, Description: "对象存储未完成分片上传（碎片）列表"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/incomplete-uploads/abort", Handler: r.abortIncompleteUploads, Description: "清理对象存储未完成分片上传"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/object-tags", Handler: r.objectTags, Description: "获取对象存储对象标签"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/object-tags", Handler: r.setObjectTags, Description: "设置对象存储对象标签"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/object-detail", Handler: r.objectDetail, Description: "获取对象存储对象详情与元数据"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/object-meta", Handler: r.updateObjectMeta, Description: "更新对象存储对象元数据"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/users", Handler: r.users, Description: "对象存储用户列表"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/users", Handler: r.createUser, Description: "创建对象存储用户"},
		apiregistry.RouteEntry{Method: "DELETE", RelativePath: "/storage/:datasourceId/users/:accessKey", Handler: r.deleteUser, Description: "删除对象存储用户"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/policies", Handler: r.policies, Description: "对象存储策略列表"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/overview", Handler: r.overview, Description: "对象存储实例概览"},
		// 大文件分片上传：单次 PUT 受反向代理请求体限制，大文件必须走分片链路
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/multipart-uploads", Handler: r.initMultipartUpload, Description: "初始化对象存储分片上传"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/multipart-uploads/part", Handler: r.uploadMultipartPart, Description: "上传对象存储分片"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/multipart-uploads/parts", Handler: r.multipartParts, Description: "列举对象存储已上传分片"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/multipart-uploads/complete", Handler: r.completeMultipartUpload, Description: "完成对象存储分片上传"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/multipart-uploads/abort", Handler: r.abortMultipartUpload, Description: "中止对象存储分片上传"},
		// Bucket 授权策略（完整 JSON，与 Bucket 配置的三档 ACL 写的是同一份数据）
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/buckets/:bucket/policy", Handler: r.getBucketPolicy, Description: "获取对象存储 Bucket 授权策略"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/buckets/:bucket/policy", Handler: r.setBucketPolicy, Description: "设置对象存储 Bucket 授权策略"},
		// 跨桶复制/移动与批量打包下载
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/objects/transfer", Handler: r.transferObjects, Description: "对象存储对象跨桶复制/移动"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/objects/archive", Handler: r.downloadObjectsArchive, Description: "对象存储批量打包下载"},
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

func (r *router) createBucket(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts createBucketOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().CreateBucket(c, m.DatasourceID, opts.Name); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) deleteBucket(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().DeleteBucket(c, m.DatasourceID, m.Bucket); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) getBucketConfig(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().GetBucketConfig(c, m.DatasourceID, m.Bucket)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) setBucketConfig(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	var cfg types.StorageBucketConfigUpdate
	if err := httputils.ShouldBindAny(c, &cfg, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().SetBucketConfig(c, m.DatasourceID, m.Bucket, &cfg); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) emptyBucket(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().EmptyBucket(c, m.DatasourceID, m.Bucket); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) getLifecycle(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().GetLifecycle(c, m.DatasourceID, m.Bucket)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) setLifecycle(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	var rules []types.StorageLifecycleRule
	if err := httputils.ShouldBindAny(c, &rules, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().SetLifecycle(c, m.DatasourceID, m.Bucket, rules); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) getCors(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().GetCors(c, m.DatasourceID, m.Bucket)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) setCors(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	var rules []types.StorageCorsRule
	if err := httputils.ShouldBindAny(c, &rules, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().SetCors(c, m.DatasourceID, m.Bucket, rules); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) objects(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts objectPageOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().ListObjectsPage(c, m.DatasourceID, opts.Bucket, opts.Prefix, opts.Token, opts.PageSize, opts.Keyword, opts.Recursive)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) uploadObject(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts uploadOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().PutObject(c, m.DatasourceID, opts.Bucket, opts.Key, c.Request.Body, c.Request.ContentLength, c.ContentType()); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) deleteObjects(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts deleteObjectsOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().DeleteObjects(c, m.DatasourceID, opts.Bucket, opts.Keys); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) renameObject(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts renameObjectOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().RenameObject(c, m.DatasourceID, opts.Bucket, opts.Src, opts.Dst); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
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
	// expires 是夹取后的生效值，不是请求里的 expiry
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) objectVersions(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts versionListOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().ListObjectVersions(c, m.DatasourceID, opts.Bucket, opts.Prefix, opts.Recycle, opts.Limit)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) bucketVersioning(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts bucketOnlyOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	state, err := r.c.Extension().Storage().GetBucketVersioningState(c, m.DatasourceID, opts.Bucket)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	// 三态返回：unknown 表示状态查询失败，前端据此提示「状态获取失败」
	// 而不是错误地断言「未开启版本控制」
	resp.Result = map[string]interface{}{
		"versioning_state":   state,
		"versioning_enabled": state == "enabled",
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) restoreObjectVersion(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts versionTargetOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().RestoreObjectVersion(c, m.DatasourceID, opts.Bucket, opts.Key, opts.VersionID); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) deleteObjectVersion(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts versionTargetOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().DeleteObjectVersion(c, m.DatasourceID, opts.Bucket, opts.Key, opts.VersionID); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) incompleteUploads(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts incompleteUploadOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().ListIncompleteUploads(c, m.DatasourceID, opts.Bucket, opts.Prefix)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) abortIncompleteUploads(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts abortUploadOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().AbortIncompleteUploads(c, m.DatasourceID, opts.Bucket, opts.Keys)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) objectTags(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts objectTagsOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().GetObjectTags(c, m.DatasourceID, opts.Bucket, opts.Key, opts.VersionID)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) setObjectTags(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts objectTagsUpdateOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().SetObjectTags(c, m.DatasourceID, opts.Bucket, opts.Key, opts.VersionID, opts.Tags); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) objectDetail(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts objectDetailOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().GetObjectDetail(c, m.DatasourceID, opts.Bucket, opts.Key, opts.VersionID)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) updateObjectMeta(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts objectMetaUpdateOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().UpdateObjectMeta(c, m.DatasourceID, opts.Bucket, opts.Key, &opts.Meta); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) users(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().ListUsers(c, m.DatasourceID)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) createUser(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts types.StorageUserCreate
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().CreateUser(c, m.DatasourceID, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) deleteUser(c *gin.Context) {
	resp := httputils.NewResponse()
	var m userMeta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().DeleteUser(c, m.DatasourceID, m.AccessKey); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) policies(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().ListPolicies(c, m.DatasourceID)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) overview(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().GetOverview(c, m.DatasourceID)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

// ---------- 大文件分片上传 ----------

func (r *router) initMultipartUpload(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts types.StorageMultipartInit
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().InitMultipartUpload(c, m.DatasourceID, &opts)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) uploadMultipartPart(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts multipartPartOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	// 分片本体直通请求体，长度取 Content-Length（浏览器上传 Blob 时会带上）
	result, err := r.c.Extension().Storage().UploadMultipartPart(
		c, m.DatasourceID, opts.Bucket, opts.Key, opts.UploadID, opts.PartNumber,
		c.Request.Body, c.Request.ContentLength,
	)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) multipartParts(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts multipartPartsOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().ListMultipartParts(c, m.DatasourceID, opts.Bucket, opts.Key, opts.UploadID)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) completeMultipartUpload(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts types.StorageMultipartComplete
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().CompleteMultipartUpload(c, m.DatasourceID, &opts)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) abortMultipartUpload(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts types.StorageMultipartTarget
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().AbortMultipartUpload(c, m.DatasourceID, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

// ---------- Bucket 授权策略 ----------

func (r *router) getBucketPolicy(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().GetBucketPolicy(c, m.DatasourceID, m.Bucket)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) setBucketPolicy(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	var opts bucketPolicyOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().Storage().SetBucketPolicy(c, m.DatasourceID, m.Bucket, opts.Policy); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

// ---------- 跨桶复制/移动与批量打包下载 ----------

func (r *router) transferObjects(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts transferObjectsOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().Storage().TransferObjects(c, m.DatasourceID, &types.StorageTransferRequest{
		Bucket:    opts.Bucket,
		Keys:      opts.Keys,
		DstBucket: opts.DstBucket,
		DstPrefix: opts.DstPrefix,
		Move:      opts.Move,
	})
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

// downloadObjectsArchive 批量/目录打包下载：响应是 zip 二进制流而非项目统一 JSON 响应。
//
// 关键点：controller 只有在「校验与展开全部成功」之后才会写出第一个字节，
// 因此 written=false 时仍能正常返回 JSON 错误；一旦开始写流，就只能靠中断连接表达失败。
func (r *router) downloadObjectsArchive(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts batchDownloadOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	filename := archiveFileName(opts.Bucket, opts.Keys)
	c.Header("Content-Type", "application/zip")
	c.Header("Cache-Control", "no-store")
	writeArchiveDisposition(c, filename)

	written, err := r.c.Extension().Storage().DownloadObjectsZip(c, m.DatasourceID, opts.Bucket, opts.Keys, c.Writer)
	if err != nil {
		klog.Errorf("download objects archive failed: %v", err)
		if !written {
			c.Header("Content-Type", "application/json; charset=utf-8")
			c.Header("Content-Disposition", "")
			httputils.SetFailed(c, resp, err)
		}
	}
}

// archiveFileName 单目录下载时用目录名，多选时用 Bucket 名
func archiveFileName(bucket string, keys []string) string {
	name := bucket + "-objects"
	if len(keys) == 1 {
		trimmed := strings.TrimSuffix(strings.TrimSpace(keys[0]), "/")
		if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
			trimmed = trimmed[idx+1:]
		}
		if trimmed != "" {
			name = trimmed
		}
	}
	return name + ".zip"
}

// 文件名里除字母数字与 -_. 之外的字符（含中文）不能直接放进 header，需转义
var archiveNameUnsafe = regexp.MustCompile(`[^\w.-]+`)

// writeArchiveDisposition 同时给出 ASCII 回退名与 RFC 5987 编码名，兼容中文目录名
func writeArchiveDisposition(c *gin.Context, filename string) {
	ascii := archiveNameUnsafe.ReplaceAllString(filename, "_")
	if strings.Trim(ascii, "_.-") == "" {
		ascii = "objects.zip"
	}
	c.Header("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, ascii, url.PathEscape(filename)))
}
