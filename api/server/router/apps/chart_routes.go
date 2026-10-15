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

// createChart 上传本地 .tgz chart 包（multipart/form-data，字段 file + category）。
func (ar *appsRouter) createChart(c *gin.Context) {
	r := httputils.NewResponse()

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	defer file.Close()

	category := c.PostForm("category")
	info, err := ar.c.Appstore().Upload(c, "", category, file)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = info
	httputils.SetSuccess(c, r)
}

// importChart 从远端 Helm 仓库导入 chart。
func (ar *appsRouter) importChart(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err error
		req types.ImportChartRequest
	)
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	info, err := ar.c.Appstore().ImportFromURL(c, &req)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = info
	httputils.SetSuccess(c, r)
}

// listCharts 分页查询 chart 目录。
func (ar *appsRouter) listCharts(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err error
		opt types.ListChartOption
	)
	if err = c.ShouldBindQuery(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	result, err := ar.c.Appstore().List(c, opt)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = result
	httputils.SetSuccess(c, r)
}

// getChart 查看 chart 详情。
func (ar *appsRouter) getChart(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err  error
		meta types.ChartIdMeta
	)
	if err = c.ShouldBindUri(&meta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	info, err := ar.c.Appstore().Get(c, meta.Id)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = info
	httputils.SetSuccess(c, r)
}

// deleteChart 删除 chart。
func (ar *appsRouter) deleteChart(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err  error
		meta types.ChartIdMeta
	)
	if err = c.ShouldBindUri(&meta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = ar.c.Appstore().Delete(c, meta.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// getChartValues 获取指定版本的默认 values。
func (ar *appsRouter) getChartValues(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err  error
		meta types.ChartIdMeta
		req  struct {
			Version string `form:"version"`
		}
	)
	if err = httputils.ShouldBindAny(c, nil, &meta, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	values, err := ar.c.Appstore().DefaultValues(c, meta.Id, req.Version)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = gin.H{"values": values}
	httputils.SetSuccess(c, r)
}

// listChartTemplates 获取 chart（最新版本）内的模板文件清单。
func (ar *appsRouter) listChartTemplates(c *gin.Context) {
	r := httputils.NewResponse()

	var meta types.ChartIdMeta
	if err := c.ShouldBindUri(&meta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	items, err := ar.c.Appstore().Templates(c, meta.Id)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = gin.H{"items": items}
	httputils.SetSuccess(c, r)
}

// getChartTemplateContent 获取指定模板文件的原始内容。
func (ar *appsRouter) getChartTemplateContent(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err  error
		meta types.ChartIdMeta
		req  struct {
			Name string `form:"name"`
		}
	)
	if err = httputils.ShouldBindAny(c, nil, &meta, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	content, err := ar.c.Appstore().TemplateContent(c, meta.Id, req.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = gin.H{"content": content}
	httputils.SetSuccess(c, r)
}
