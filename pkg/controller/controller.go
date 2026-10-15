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

package controller

import (
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/controller/agent"
	"github.com/caoyingjunz/pixiu/pkg/controller/alert"
	"github.com/caoyingjunz/pixiu/pkg/controller/apiresource"
	"github.com/caoyingjunz/pixiu/pkg/controller/apps"
	"github.com/caoyingjunz/pixiu/pkg/controller/assistant"
	"github.com/caoyingjunz/pixiu/pkg/controller/audit"
	"github.com/caoyingjunz/pixiu/pkg/controller/auth"
	"github.com/caoyingjunz/pixiu/pkg/controller/cluster"
	"github.com/caoyingjunz/pixiu/pkg/controller/datasource"
	"github.com/caoyingjunz/pixiu/pkg/controller/distribution"
	"github.com/caoyingjunz/pixiu/pkg/controller/email"
	extensioncontroller "github.com/caoyingjunz/pixiu/pkg/controller/extension"
	"github.com/caoyingjunz/pixiu/pkg/controller/helm"
	"github.com/caoyingjunz/pixiu/pkg/controller/node"
	"github.com/caoyingjunz/pixiu/pkg/controller/plan"
	"github.com/caoyingjunz/pixiu/pkg/controller/role"
	"github.com/caoyingjunz/pixiu/pkg/controller/runner"
	"github.com/caoyingjunz/pixiu/pkg/controller/tenant"
	"github.com/caoyingjunz/pixiu/pkg/controller/user"
	"github.com/caoyingjunz/pixiu/pkg/db"
)

type PixiuInterface interface {
	cluster.ClusterGetter
	tenant.TenantGetter
	role.RoleGetter
	apiresource.APIResourceGetter
	user.UserGetter
	plan.PlanGetter
	distribution.DistributionGetter
	node.NodeGetter
	audit.AuditGetter
	helm.HelmGetter
	agent.AgentGetter
	datasource.Getter
	runner.RunnerGetter
	assistant.Getter
	alert.Getter
	email.Getter
	auth.Getter
	extensioncontroller.Getter
	apps.Getter
}

type pixiu struct {
	cc      config.Config
	factory db.ShareDaoFactory

	// extension 扩展能力父接口，内部持有 redis 等子模块的单例 controller
	extension extensioncontroller.Interface
}

func (p *pixiu) Cluster() cluster.Interface { return cluster.NewCluster(p.cc, p.factory) }
func (p *pixiu) Tenant() tenant.Interface   { return tenant.NewTenant(p.cc, p.factory) }
func (p *pixiu) Role() role.Interface       { return role.NewRole(p.cc, p.factory) }
func (p *pixiu) APIResource() apiresource.Interface {
	return apiresource.NewAPIResource(p.cc, p.factory)
}
func (p *pixiu) User() user.Interface             { return user.NewUser(p.cc, p.factory) }
func (p *pixiu) Plan() plan.Interface             { return plan.NewPlan(p.cc, p.factory) }
func (p *pixiu) Node() node.Interface             { return node.NewNode(p.cc, p.factory) }
func (p *pixiu) Audit() audit.Interface           { return audit.NewAudit(p.cc, p.factory) }
func (p *pixiu) Helm() helm.Interface             { return helm.NewHelm(p.factory) }
func (p *pixiu) Agent() agent.Interface           { return agent.NewAgent(p.cc, p.factory) }
func (p *pixiu) Datasource() datasource.Interface { return datasource.New(p.cc, p.factory) }
func (p *pixiu) Assistant() assistant.Interface   { return assistant.New(p.cc, p.factory) }
func (p *pixiu) Alert() alert.Interface           { return alert.New(p.cc, p.factory) }
func (p *pixiu) Email() email.Interface {
	return email.New(p.cc, p.factory)
}
func (p *pixiu) Auth() auth.Interface {
	return auth.New(p.cc, p.factory)
}
func (p *pixiu) Distribution() distribution.Interface {
	return distribution.NewDistribution(p.cc, p.factory)
}
func (p *pixiu) Runner() runner.Interface { return runner.NewRunner(p.cc, p.factory) }
func (p *pixiu) Appstore() apps.Interface {
	return apps.New(p.cc, p.factory)
}
func (p *pixiu) Extension() extensioncontroller.Interface {
	return p.extension
}

func New(cfg config.Config, f db.ShareDaoFactory) PixiuInterface {
	return &pixiu{
		cc:        cfg,
		factory:   f,
		extension: extensioncontroller.New(cfg, f),
	}
}
