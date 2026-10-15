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

package cluster

import (
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const (
	kubeProxyBaseURL = "/pixiu/kubeproxy"
	indexerBaseURL   = "/pixiu/indexer"
)

// clusterRouter is a router to talk with the cluster controller
type clusterRouter struct {
	c controller.PixiuInterface
}

// NewRouter initializes a new cluster router
func NewRouter(o *options.Options) {
	s := &clusterRouter{
		c: o.Controller,
	}
	s.initRoutes(o.HttpEngine)
}

func (cr *clusterRouter) initRoutes(ginEngine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "集群管理",
		BaseURL: "/pixiu/clusters",
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: cr.createCluster, Description: "创建集群"},
			{Method: "PUT", RelativePath: "/:clusterId", Handler: cr.updateCluster, Description: "更新集群"},
			{Method: "DELETE", RelativePath: "/:clusterId", Handler: cr.deleteCluster, Description: "删除集群"},
			{Method: "GET", RelativePath: "/:clusterId", Handler: cr.getCluster, Description: "查看详情"},
			{Method: "GET", RelativePath: "", Handler: cr.listClusters, Description: "查看列表"},

			{Method: "POST", RelativePath: "/ping", Handler: cr.pingCluster, Description: "连通测试"},
			{Method: "POST", RelativePath: "/protect/:clusterId", Handler: cr.protectCluster, Description: "删除保护"},
			{Method: "GET", RelativePath: "/:clusterId/kubeconfig", Handler: cr.getClusterKubeconfig, Description: "查看KubeConfig"},

			{Method: "POST", RelativePath: "/:clusterId/proxy-kubeconfig", Handler: cr.createProxyKubeconfig, Description: "生成代理KubeConfig"},
			{Method: "GET", RelativePath: "/:clusterId/proxy-kubeconfig", Handler: cr.getProxyKubeconfig, Description: "获取代理KubeConfig"},
			{Method: "DELETE", RelativePath: "/:clusterId/access-tokens/:jti", Handler: cr.revokeAccessToken, Description: "删除访问令牌"},
		},
	}
	group.Register(ginEngine.Group("/pixiu/clusters"), cr.c.APIResource())

	// 集群权限
	permGroup := &apiregistry.Group{
		Name:    "集群权限",
		BaseURL: "/pixiu/clusters",
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "/:clusterId/permissions", Handler: cr.createPermission, Description: "创建权限"},
			{Method: "GET", RelativePath: "/permissions", Handler: cr.listPermissions, Description: "权限列表"},
			{Method: "GET", RelativePath: "/permissions/:permissionId", Handler: cr.getPermission, Description: "查看权限"},
			{Method: "PUT", RelativePath: "/permissions/:permissionId", Handler: cr.updatePermission, Description: "更新权限"},
			{Method: "DELETE", RelativePath: "/permissions/:permissionId", Handler: cr.deletePermission, Description: "删除权限"},
		},
	}
	permGroup.Register(ginEngine.Group("/pixiu/clusters"), cr.c.APIResource())

	// 集群代理
	proxyGroup := &apiregistry.Group{
		Name:    "集群代理",
		BaseURL: kubeProxyBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "GET", RelativePath: "/clusters/:cluster/namespaces/:namespace/pods/:pod/log", Handler: cr.watchPodLog, Description: "Pod日志"},
			{Method: "GET", RelativePath: "/clusters/:cluster/namespaces/:namespace/name/:name/kind/:kind/events", Handler: cr.aggregateEvents, Description: "聚合事件"},
			//{Method: "GET", RelativePath: "/clusters/:cluster/api/v1/events", Handler: cr.getEventList, Description: "事件列表"},
			{Method: "POST", RelativePath: "/clusters/:cluster/namespaces/:namespace/jobs/:name", Handler: cr.ReRunJob, Description: "重新执行Job"},
			// ws 相关入口
			{Method: "GET", RelativePath: "/pods/ws", Handler: cr.podWebShell, Description: "Pod WebShell"},
			{Method: "GET", RelativePath: "/nodes/ws", Handler: cr.nodeWebShell, Description: "Node WebShell"},
			{Method: "GET", RelativePath: "/clusters/ws", Handler: cr.clusterWebShell, Description: "Cluster WebShell"},

			{Method: "GET", RelativePath: "/clusters/:cluster/namespaces/:namespace/pods/:pod/files", Handler: cr.listPodFiles, Description: "Pod文件列表"},
			{Method: "GET", RelativePath: "/clusters/:cluster/namespaces/:namespace/pods/:pod/files/download", Handler: cr.downloadPodFile, Description: "下载Pod文件"},
			{Method: "POST", RelativePath: "/clusters/:cluster/namespaces/:namespace/pods/:pod/files/upload", Handler: cr.uploadPodFile, Description: "上传Pod文件"},
		},
	}
	proxyGroup.Register(ginEngine.Group(kubeProxyBaseURL), cr.c.APIResource())
}
