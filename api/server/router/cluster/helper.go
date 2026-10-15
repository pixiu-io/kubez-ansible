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

package cluster

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
)

func IsKubeProxyPath(c *gin.Context) bool {
	return strings.HasPrefix(c.Request.URL.Path, kubeProxyBaseURL)
}

// resolveCredClusterName 按请求集群名鉴权，返回实际用于加载 kubeconfig 的集群名。
// 主集群名请求且用户不能使用 admin kubeconfig 时，回落到其授权子集群名。
func (cr *clusterRouter) resolveCredClusterName(c *gin.Context, clusterName string) (string, error) {
	user, err := httputils.GetUserFromContext(c)
	if err != nil {
		return "", err
	}
	credCluster, err := cr.c.Cluster().AuthorizeClusterAccessByName(c, user, clusterName)
	if err != nil {
		return "", err
	}
	return credCluster.Name, nil
}

// authorizeClusterAccess 按 id 做资源访问校验（不含主集群 kubeconfig 归属限制），用于 CloudShell 等。
func (cr *clusterRouter) authorizeClusterAccess(c *gin.Context, clusterId int64) error {
	user, err := httputils.GetUserFromContext(c)
	if err != nil {
		return err
	}
	_, err = cr.c.Cluster().AuthorizeClusterAccess(c, user, clusterId)
	return err
}
