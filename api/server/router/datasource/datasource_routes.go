/*
Copyright 2024 The Pixiu Authors.

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

package datasource

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type meta struct {
	DatasourceId int64 `uri:"datasourceId"`
}

func (dr *datasourceRouter) createDatasource(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		req types.CreateDatasourceRequest
		err error
	)
	if err = httputils.BindCreateRequest(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = dr.c.Datasource().Create(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (dr *datasourceRouter) updateDatasource(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		idMeta meta
		req    types.UpdateDatasourceRequest
		err    error
	)
	if err = httputils.ShouldBindAny(c, &req, &idMeta, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	req.Id = idMeta.DatasourceId
	if err = dr.c.Datasource().Update(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (dr *datasourceRouter) deleteDatasource(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = dr.c.Datasource().Delete(c, m.DatasourceId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (dr *datasourceRouter) getDatasource(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = dr.c.Datasource().Get(c, m.DatasourceId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	// 凭据（对象存储 AK/SK、各类型数据源密码）不对外返回：任何有该数据源读权限的用户
	// 都不应拿到明文。编辑表单对密钥采用「留空表示不修改」语义，Update 会用旧值回填。
	// 脱敏只在这一层做，控制器内部调用方（如代理透传鉴权）仍取到明文。
	if ds, ok := r.Result.(*types.Datasource); ok {
		ds.Config.MaskSensitiveFields()
	}
	httputils.SetSuccess(c, r)
}

func (dr *datasourceRouter) listDatasources(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		listOption types.ListOptions
		err        error
	)
	if err = httputils.BindListOptionsWithUser(c, &listOption); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	result, err := dr.c.Datasource().List(c, listOption)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	// 列表同样脱敏，规则见 getDatasource
	if pageResult, ok := result.(types.PageResult); ok {
		if items, ok := pageResult.Items.([]types.Datasource); ok {
			for i := range items {
				items[i].Config.MaskSensitiveFields()
			}
			pageResult.Items = items
		}
		result = pageResult
	}
	r.Result = result
	httputils.SetSuccess(c, r)
}
