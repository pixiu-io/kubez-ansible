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

package db

import (
	"context"

	"gorm.io/gorm"
)

type ShareDaoFactory interface {
	Cluster() ClusterInterface
	Tenant() TenantInterface
	Role() RoleInterface
	API() APIResourceInterface
	User() UserInterface
	Plan() PlanInterface
	Distribution() DistributionInterface
	Audit() AuditInterface
	Agent() AgentInterface
	Permission() PermissionInterface
	Datasource() DatasourceInterface
	Runner() RunnerInterface
	Assistant() AssistantInterface
	Alert() AlertInterface
	Email() EmailInterface
	RegistrationCode() RegistrationCodeInterface
	CronHpa() CronHpaInterface
	Chart() ChartInterface

	// Transaction 在单库事务中执行 fn，fn 内应使用传入的 factory 访问 DAO。
	Transaction(ctx context.Context, fn func(ShareDaoFactory) error) error
}

type shareDaoFactory struct {
	db *gorm.DB
}

func (f *shareDaoFactory) Cluster() ClusterInterface           { return newCluster(f.db) }
func (f *shareDaoFactory) Tenant() TenantInterface             { return newTenant(f.db) }
func (f *shareDaoFactory) Role() RoleInterface                 { return newRole(f.db) }
func (f *shareDaoFactory) API() APIResourceInterface           { return newAPIs(f.db) }
func (f *shareDaoFactory) User() UserInterface                 { return newUser(f.db) }
func (f *shareDaoFactory) Plan() PlanInterface                 { return newPlan(f.db) }
func (f *shareDaoFactory) Distribution() DistributionInterface { return newDistribution(f.db) }
func (f *shareDaoFactory) Audit() AuditInterface               { return newAudit(f.db) }
func (f *shareDaoFactory) Agent() AgentInterface               { return newAgent(f.db) }
func (f *shareDaoFactory) Permission() PermissionInterface     { return newPermission(f.db) }
func (f *shareDaoFactory) Datasource() DatasourceInterface     { return newDatasource(f.db) }
func (f *shareDaoFactory) Runner() RunnerInterface             { return newRunner(f.db) }
func (f *shareDaoFactory) Assistant() AssistantInterface       { return newAssistant(f.db) }
func (f *shareDaoFactory) Alert() AlertInterface               { return newAlert(f.db) }
func (f *shareDaoFactory) Email() EmailInterface               { return newEmail(f.db) }
func (f *shareDaoFactory) RegistrationCode() RegistrationCodeInterface {
	return newRegistrationCode(f.db)
}
func (f *shareDaoFactory) CronHpa() CronHpaInterface { return newCronHpa(f.db) }
func (f *shareDaoFactory) Chart() ChartInterface     { return newChart(f.db) }

func (f *shareDaoFactory) Transaction(ctx context.Context, fn func(ShareDaoFactory) error) error {
	return f.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&shareDaoFactory{db: tx})
	})
}

func NewDaoFactory(db *gorm.DB, migrate bool) (ShareDaoFactory, error) {
	if migrate {
		// 自动创建指定模型的数据库表结构
		if err := newMigrator(db).AutoMigrate(); err != nil {
			return nil, err
		}
	}

	return &shareDaoFactory{
		db: db,
	}, nil
}
