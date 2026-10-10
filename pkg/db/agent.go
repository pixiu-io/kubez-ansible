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

package db

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type AgentInterface interface {
	Create(ctx context.Context, object *model.Agent) (*model.Agent, error)
	Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*model.Agent, error)
	List(ctx context.Context, opts ...Options) ([]model.Agent, error)
	Count(ctx context.Context, opts ...Options) (int64, error)

	GetBy(ctx context.Context, opts ...Options) (*model.Agent, error)
	InternalUpdate(ctx context.Context, id int64, updates map[string]interface{}) error
	Job() JobInterface
}

type agent struct{ db *gorm.DB }

// WithToken 按 token 过滤；注意：token 为空时不追加任何 WHERE 条件（返回未过滤查询），
// 鉴权入口必须先拒绝空 token（见 agentController.getAuthAgent），不得依赖本选项做空值防护
func WithToken(token string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if token == "" {
			return tx
		}
		return tx.Where("token = ?", token)
	}
}

func newAgent(db *gorm.DB) AgentInterface { return &agent{db: db} }

func (a *agent) Job() JobInterface { return newJob(a.db) }

func (a *agent) Create(ctx context.Context, object *model.Agent) (*model.Agent, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	object.LastHeartbeat = now
	if err := a.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (a *agent) Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["resource_version"] = resourceVersion + 1
	updates["gmt_modified"] = time.Now()
	f := a.db.WithContext(ctx).Model(&model.Agent{}).
		Where("id = ? AND resource_version = ?", id, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotUpdate
	}
	return nil
}

func (a *agent) InternalUpdate(ctx context.Context, id int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	return a.db.WithContext(ctx).Model(&model.Agent{}).Where("id = ?", id).Updates(updates).Error
}

func (a *agent) Delete(ctx context.Context, id int64) error {
	return a.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Agent{}).Error
}

func (a *agent) Get(ctx context.Context, id int64) (*model.Agent, error) {
	var object model.Agent
	if err := a.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (a *agent) GetBy(ctx context.Context, opts ...Options) (*model.Agent, error) {
	var object model.Agent
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (a *agent) List(ctx context.Context, opts ...Options) ([]model.Agent, error) {
	var objects []model.Agent
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (a *agent) Count(ctx context.Context, opts ...Options) (int64, error) {
	var total int64
	tx := a.db.WithContext(ctx).Model(&model.Agent{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	return total, tx.Count(&total).Error
}
