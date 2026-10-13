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

// Package extension 扩展管理路由组，子模块（redis、mysql、autoscaling 等）注册到该组下
package extension

import (
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	autoscalingrouter "github.com/caoyingjunz/pixiu/api/server/router/extension/autoscaling"
	mysqlrouter "github.com/caoyingjunz/pixiu/api/server/router/extension/mysql"
	postgresrouter "github.com/caoyingjunz/pixiu/api/server/router/extension/postgres"
	redisrouter "github.com/caoyingjunz/pixiu/api/server/router/extension/redis"
	storagerouter "github.com/caoyingjunz/pixiu/api/server/router/extension/storage"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
)

const extensionBaseURL = "/pixiu/extension"

// NewRouter 扩展管理路由组，子模块（redis、mysql、autoscaling 等）注册到该组下
func NewRouter(o *options.Options) {
	group := &apiregistry.Group{
		Name:    "扩展管理",
		BaseURL: extensionBaseURL,
	}
	autoscalingrouter.RegisterAutoscaling(o, group)
	redisrouter.RegisterRedis(o, group)
	mysqlrouter.RegisterMysql(o, group)
	postgresrouter.RegisterPostgres(o, group)
	storagerouter.RegisterStorage(o, group)
	group.Register(o.HttpEngine.Group(extensionBaseURL), o.Controller.APIResource())
}
