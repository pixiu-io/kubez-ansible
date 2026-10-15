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
	"strings"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/util"
)

var alwaysAllowPath sets.String

func init() {
	alwaysAllowPath = sets.NewString(
		"/pixiu/auth/login",
		"/pixiu/auth/verification-codes",
		"/pixiu/auth/register",
		"/pixiu/auth/forgot-password/verification-codes",
		"/pixiu/auth/reset-password",
		"/pixiu/connect",
	)
}

// 允许特定请求不经过 JWT 验证（由业务侧 Token 鉴权）
func allowCustomRequest(c *gin.Context) bool {
	path := c.Request.URL.Path
	// Agent 任务 API（deploy-agent heartbeat / claim / logs / result / plan）
	if strings.HasPrefix(path, "/pixiu/agents/heartbeat") ||
		strings.HasPrefix(path, "/pixiu/agents/claim") ||
		strings.HasPrefix(path, "/pixiu/agents/jobs/") {
		return true
	}
	return false
}

func InstallMiddlewares(o *options.Options) {
	// 依次进行跨域，日志，登录限流，单用户限速，总量限速，验证，鉴权和审计
	o.HttpEngine.Use(
		requestid.New(requestid.WithGenerator(func() string {
			return util.GenerateRequestID()
		})),
		Cors(),
		Logger(&o.ComponentConfig.Log),
		LoginRateLimiter(),
		RegistrationRateLimiter(),
		UserRateLimiter(),
		Limiter(),
		Authentication(o),
		Authorization(o),
		Admission(),
		Audit(o),
	)
}
