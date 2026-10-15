/*
Copyright 2021 The Pixiu Authors.

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

package router

import (
	"embed"
	"net/http"

	alertrouter "github.com/caoyingjunz/pixiu/api/server/router/alert"
	assistant "github.com/caoyingjunz/pixiu/api/server/router/assistant"
	datasource "github.com/caoyingjunz/pixiu/api/server/router/datasource"
	"github.com/gin-gonic/gin"

	_ "github.com/caoyingjunz/pixiu/api/server/validator"

	"github.com/caoyingjunz/pixiu/api/server/middleware"
	"github.com/caoyingjunz/pixiu/api/server/router/agent"
	"github.com/caoyingjunz/pixiu/api/server/router/apiresource"
	appsrouter "github.com/caoyingjunz/pixiu/api/server/router/apps"
	"github.com/caoyingjunz/pixiu/api/server/router/audit"
	authrouter "github.com/caoyingjunz/pixiu/api/server/router/auth"
	"github.com/caoyingjunz/pixiu/api/server/router/cluster"
	"github.com/caoyingjunz/pixiu/api/server/router/distribution"
	emailrouter "github.com/caoyingjunz/pixiu/api/server/router/email"
	extensionrouter "github.com/caoyingjunz/pixiu/api/server/router/extension"
	"github.com/caoyingjunz/pixiu/api/server/router/node"
	"github.com/caoyingjunz/pixiu/api/server/router/plan"
	"github.com/caoyingjunz/pixiu/api/server/router/proxy"
	"github.com/caoyingjunz/pixiu/api/server/router/role"
	"github.com/caoyingjunz/pixiu/api/server/router/runner"
	"github.com/caoyingjunz/pixiu/api/server/router/tenant"
	"github.com/caoyingjunz/pixiu/api/server/router/tunnel"
	"github.com/caoyingjunz/pixiu/api/server/router/user"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/static"
)

type RegisterFunc func(o *options.Options)

//go:embed static
var EmbedFS embed.FS

func InstallRouters(o *options.Options) {
	fs := []RegisterFunc{
		middleware.InstallMiddlewares,
		cluster.NewRouter,
		appsrouter.NewRouter,
		proxy.NewRouter,
		tenant.NewRouter,
		role.NewRouter,
		apiresource.NewRouter,
		authrouter.NewRouter,
		user.NewRouter,
		distribution.NewRouter,
		plan.NewRouter,
		node.NewRouter,
		audit.NewRouter,
		agent.NewRouter,
		datasource.NewRouter,
		runner.NewRouter,
		assistant.NewRouter,
		alertrouter.NewRouter,
		emailrouter.NewRouter,
		tunnel.NewRouter,
		extensionrouter.NewRouter,
	}

	install(o, fs...)

	// StaticFiles 启用前端集成
	o.HttpEngine.Use(static.Serve("/", static.LocalFile(o.ComponentConfig.Default.StaticFiles, true)))

	// 启动健康检查
	o.HttpEngine.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
}

func install(o *options.Options, fs ...RegisterFunc) {
	for _, f := range fs {
		f(o)
	}
}
