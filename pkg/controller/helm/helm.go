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

package helm

import (
	"context"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/controller/cluster"
	"github.com/caoyingjunz/pixiu/pkg/db"
)

// AllNamespacesToken 是「全部命名空间」的哨兵值，由前端在请求路径中传入
// (GET .../namespaces/<token>/releases)。k8s 命名空间名不允许下划线，
// 因此该值与任何真实命名空间都不会冲突。
const AllNamespacesToken = "_all"

type HelmGetter interface {
	Helm() Interface
}

type Interface interface {
	Release(cluster, namespace string) ReleaseInterface
}

type Helm struct {
	factory db.ShareDaoFactory
}

func (h *Helm) Release(cluster, namespace string) ReleaseInterface {
	cs := h.MustGetClusterSetByName(context.Background(), cluster)
	settings := cli.New()
	settings.SetNamespace(namespace)
	actionConfig := new(action.Configuration)
	resetClientGetter := client.NewHelmRESTClientGetter(cs.Config)

	// storage driver 在 Init 时按命名空间绑定：secrets driver 内部持有
	// Secrets(namespace) 客户端，List 时据此列举。要真正跨命名空间列举，
	// 必须用「空命名空间」Init（空命名空间下 Secrets("").List() 走 all-namespaces 查询），
	// 这与 helm CLI `list --all-namespaces` 重新 Init 的机制一致。
	// 详见 pkg/controller/helm/releases.go List() 中的说明。
	initNamespace := settings.Namespace()
	if initNamespace == AllNamespacesToken {
		initNamespace = ""
	}
	actionConfig.Init(
		resetClientGetter,
		initNamespace,
		"secrets",
		klog.Infof,
	)
	return NewReleases(actionConfig, settings)
}

func NewHelm(factory db.ShareDaoFactory) Interface {
	return &Helm{
		factory: factory,
	}
}

func (h *Helm) MustGetClusterSetByName(ctx context.Context, name string) client.ClusterSet {
	cs, ok := cluster.ClusterIndexer.Get(name)
	if ok {
		klog.Infof("Get %s clusterSet from indexer", name)
		return cs
	}

	klog.Infof("building clusterSet for %s", name)
	// 缓存中不存在，则新建并重写回缓存
	object, err := h.factory.Cluster().GetBy(ctx, db.WithName(name))
	if err != nil {
		return client.ClusterSet{}
	}
	if object == nil {
		return client.ClusterSet{}
	}
	newClusterSet, err := client.NewClusterSet(object.KubeConfig)
	if err != nil {
		return client.ClusterSet{}
	}

	klog.Infof("set %s clusterSet into indexer", name)
	cluster.ClusterIndexer.Set(name, *newClusterSet)
	return *newClusterSet
}
