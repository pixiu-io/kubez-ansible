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

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const appsBaseURL = "/pixiu/apps"

type appsRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	ar := &appsRouter{
		c: o.Controller,
	}
	ar.initRoutes(o.HttpEngine)
}

func (ar *appsRouter) initRoutes(ginEngine *gin.Engine) {
	persist := true

	group := &apiregistry.Group{
		Name:    "应用资源",
		BaseURL: appsBaseURL,
		Entries: []apiregistry.RouteEntry{
			// chart 目录
			{Method: "POST", RelativePath: "/charts", Handler: ar.createChart, Description: "上传Chart", Persist: &persist},
			{Method: "GET", RelativePath: "/charts", Handler: ar.listCharts, Description: "Chart列表", Persist: &persist},
			{Method: "GET", RelativePath: "/charts/:id", Handler: ar.getChart, Description: "Chart详情", Persist: &persist},
			{Method: "DELETE", RelativePath: "/charts/:id", Handler: ar.deleteChart, Description: "删除Chart", Persist: &persist},
			{Method: "GET", RelativePath: "/charts/:id/values", Handler: ar.getChartValues, Description: "获取配置值", Persist: &persist},
			{Method: "GET", RelativePath: "/charts/:id/templates", Handler: ar.listChartTemplates, Description: "模板列表", Persist: &persist},
			{Method: "GET", RelativePath: "/charts/:id/templates/content", Handler: ar.getChartTemplateContent, Description: "模板内容", Persist: &persist},

			// release（集群作用域自然身份：cluster + namespace + name）
			{Method: "POST", RelativePath: "/clusters/:cluster/namespaces/:namespace/releases", Handler: ar.installRelease, Description: "安装Release", Persist: &persist},
			{Method: "GET", RelativePath: "/clusters/:cluster/releases", Handler: ar.listReleases, Description: "Release列表(全部命名空间)", Persist: &persist},
			{Method: "GET", RelativePath: "/clusters/:cluster/namespaces/:namespace/releases", Handler: ar.listReleases, Description: "Release列表(指定命名空间)", Persist: &persist},
			{Method: "GET", RelativePath: "/clusters/:cluster/namespaces/:namespace/releases/:name", Handler: ar.getRelease, Description: "Release详情", Persist: &persist},
			{Method: "PUT", RelativePath: "/clusters/:cluster/namespaces/:namespace/releases/:name", Handler: ar.upgradeRelease, Description: "升级Release", Persist: &persist},
			{Method: "DELETE", RelativePath: "/clusters/:cluster/namespaces/:namespace/releases/:name", Handler: ar.uninstallRelease, Description: "卸载Release", Persist: &persist},
			{Method: "GET", RelativePath: "/clusters/:cluster/namespaces/:namespace/releases/:name/history", Handler: ar.releaseHistory, Description: "Release历史", Persist: &persist},
			{Method: "POST", RelativePath: "/clusters/:cluster/namespaces/:namespace/releases/:name/rollback", Handler: ar.rollbackRelease, Description: "回滚Release", Persist: &persist},
			{Method: "GET", RelativePath: "/clusters/:cluster/namespaces/:namespace/releases/:name/logs", Handler: ar.releaseLogs, Description: "Release日志", Persist: &persist},
		},
	}
	group.Register(ginEngine.Group(appsBaseURL), ar.c.APIResource())
}
