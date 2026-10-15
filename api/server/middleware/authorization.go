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

package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/api/server/router/cluster"
	"github.com/caoyingjunz/pixiu/api/server/router/proxy"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// Authorization 鉴权（用户状态与只读模式）；细粒度 RBAC 已移除。
func Authorization(o *options.Options) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 允许请求直接通过
		if o.ComponentConfig.Default.Mode.InDebug() || alwaysAllowPath.Has(c.Request.URL.Path) || allowCustomRequest(c) {
			return
		}

		user, err := httputils.GetUserFromContext(c)
		if err != nil {
			httputils.AbortFailedWithCode(c, http.StatusMethodNotAllowed, err)
			return
		}

		// 0 正常  1 禁用
		switch user.Status {
		case model.UserStatusForbidden:
			// 禁用用户无法进行任何操作
			httputils.AbortFailedWithCode(c, http.StatusForbidden, fmt.Errorf("用户已被禁用"))
			return
		}

		// Proxy / kube gateway path should be skipped now.
		// TODO: get object and ID from proxy path
		if proxy.IsProxyPath(c) || proxy.IsKubeGatewayPath(c) || cluster.IsKubeProxyPath(c) {
			c.Next()
			return
		}
	}
}
