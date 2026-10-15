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

package apps

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// installRelease 安装 release 到集群命名空间（集群/命名空间由 URI 定位，chart 由 body 指定）。
func (ar *appsRouter) installRelease(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err  error
		meta types.ReleaseNamespaceMeta
		req  types.InstallReleaseRequest
	)
	if err = httputils.ShouldBindAny(c, &req, &meta, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	info, err := ar.c.Appstore().Install(c, meta.Cluster, meta.Namespace, &req)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = info
	httputils.SetSuccess(c, r)
}

// listReleases 列出集群内 release（namespace 为空表示全部命名空间）。
func (ar *appsRouter) listReleases(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err  error
		meta types.ReleaseNamespaceMeta
	)
	if err = c.ShouldBindUri(&meta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	items, err := ar.c.Appstore().ListReleases(c, meta.Cluster, meta.Namespace)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = items
	httputils.SetSuccess(c, r)
}

// getRelease 查看单个 release 详情。
func (ar *appsRouter) getRelease(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err  error
		meta types.ReleasePathMeta
	)
	if err = c.ShouldBindUri(&meta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	info, err := ar.c.Appstore().GetRelease(c, meta.Cluster, meta.Namespace, meta.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = info
	httputils.SetSuccess(c, r)
}

// upgradeRelease 升级已安装 release。
func (ar *appsRouter) upgradeRelease(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err  error
		meta types.ReleasePathMeta
		req  types.UpgradeChartRequest
	)
	if err = httputils.ShouldBindAny(c, &req, &meta, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = ar.c.Appstore().Upgrade(c, meta.Cluster, meta.Namespace, meta.Name, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// uninstallRelease 卸载已安装 release。
func (ar *appsRouter) uninstallRelease(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err  error
		meta types.ReleasePathMeta
	)
	if err = c.ShouldBindUri(&meta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = ar.c.Appstore().Uninstall(c, meta.Cluster, meta.Namespace, meta.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// releaseHistory 查询 release 历史。
func (ar *appsRouter) releaseHistory(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err  error
		meta types.ReleasePathMeta
	)
	if err = c.ShouldBindUri(&meta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	items, err := ar.c.Appstore().History(c, meta.Cluster, meta.Namespace, meta.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = items
	httputils.SetSuccess(c, r)
}

// rollbackRelease 回滚 release 到指定 revision。
func (ar *appsRouter) rollbackRelease(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err  error
		meta types.ReleasePathMeta
		req  types.RollbackChartRequest
	)
	if err = httputils.ShouldBindAny(c, &req, &meta, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = ar.c.Appstore().Rollback(c, meta.Cluster, meta.Namespace, meta.Name, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// releaseLogs SSE 实时推送单条 release 的执行日志（先回放、再 follow，客户端断开即退出）。
func (ar *appsRouter) releaseLogs(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err  error
		meta types.ReleasePathMeta
	)
	if err = c.ShouldBindUri(&meta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = ar.c.Appstore().WatchLogs(c, meta.Cluster, meta.Namespace, meta.Name, c.Writer, c.Request); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
}
