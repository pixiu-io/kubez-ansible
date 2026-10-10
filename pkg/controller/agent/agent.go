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

package agent

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/controller/plan"
	"github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util/token"
)

var TokenHeader = token.AgentTokenHeader

type AgentGetter interface {
	Agent() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateAgentRequest) error
	Update(ctx context.Context, agentId int64, req *types.UpdateAgentRequest) error
	Delete(ctx context.Context, agentId int64) error
	Get(ctx context.Context, agentId int64) (*types.Agent, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)

	Heartbeat(ctx context.Context, agentToken string, req *types.AgentHeartbeatRequest) error
	Claim(ctx context.Context, agentToken string) (*types.Job, error)

	AppendLogs(ctx context.Context, agentToken string, jobId int64, req *types.AgentJobLogsRequest) error
	ReportResult(ctx context.Context, agentToken string, jobId int64, req *types.AgentJobResultRequest) error
	GetPlan(ctx context.Context, agentToken string, jobId int64) (*types.Plan, error)
}

type agentController struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func NewAgent(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &agentController{
		cc:      cfg,
		factory: f,
	}
}

func (a *agentController) Create(ctx context.Context, req *types.CreateAgentRequest) error {
	tkn, err := token.Generate()
	if err != nil {
		return errors.ErrServerInternal
	}
	_, err = a.factory.Agent().Create(ctx, &model.Agent{
		Name:        req.Name,
		AgentType:   req.Type,
		UserID:      req.UserID,
		Token:       tkn,
		Status:      model.AgentStatusOffline,
		Description: req.Description,
	})
	if err != nil {
		klog.Errorf("failed to create agent %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}
	return nil
}

// 更新前置检查：资源存在 + 非超级管理员只能更新自己的 Agent
func (a *agentController) preUpdate(ctx context.Context, agentId int64) error {
	object, err := a.factory.Agent().Get(ctx, agentId)
	if err != nil {
		klog.Errorf("failed to get agent(%d): %v", agentId, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrAgentNotFound
	}
	if err := util.CheckResourceAccess(ctx, a.factory, object.UserID, types.ResourceTypeAgent, agentId); err != nil {
		return err
	}
	return nil
}

func (a *agentController) Update(ctx context.Context, agentId int64, req *types.UpdateAgentRequest) error {
	if err := a.preUpdate(ctx, agentId); err != nil {
		klog.Errorf("pre-update check failed for agent(%d): %v", agentId, err)
		return err
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Type != nil {
		updates["agent_type"] = *req.Type
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if err := a.factory.Agent().Update(ctx, agentId, req.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to update agent %d: %v", agentId, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (a *agentController) Delete(ctx context.Context, agentId int64) error {
	// 非超级管理员只能删除自己的 Agent
	object, err := a.factory.Agent().Get(ctx, agentId)
	if err != nil {
		klog.Errorf("failed to get agent(%d): %v", agentId, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrAgentNotFound
	}
	if err = util.CheckResourceAccess(ctx, a.factory, object.UserID, types.ResourceTypeAgent, agentId); err != nil {
		return err
	}

	if err := a.factory.Agent().Delete(ctx, agentId); err != nil {
		klog.Errorf("failed to delete agent %d: %v", agentId, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (a *agentController) Get(ctx context.Context, agentId int64) (*types.Agent, error) {
	object, err := a.factory.Agent().Get(ctx, agentId)
	if err != nil {
		klog.Errorf("failed to get agent(%d): %v", agentId, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrAgentNotFound
	}

	// 非超级管理员只能查看自己的 Agent 或被 scope 授权的 Agent
	if err = util.CheckResourceAccess(ctx, a.factory, object.UserID, types.ResourceTypeAgent, agentId); err != nil {
		return nil, err
	}

	return model2Type(object), nil
}

func (a *agentController) List(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	// 资源级授权：非超管用户叠加 scope 授权的 agent id（超管走 listOption.UserId 现状即可）
	authorizedAgentIDs, err := util.AuthorizedResourceIDs(ctx, a.factory, types.ResourceTypeAgent)
	if err != nil {
		return nil, err
	}

	filterOpts := buildAgentFilters(listOption.UserId, listOption, authorizedAgentIDs)

	pageResult.Total, err = a.factory.Agent().Count(ctx, filterOpts...)
	if err != nil {
		klog.Errorf("failed to get agents count: %v", err)
		return nil, err
	}

	offset := (listOption.Page - 1) * listOption.Limit
	paginationOpts := append(filterOpts,
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
		db.WithOrderByDesc(),
	)

	objects, err := a.factory.Agent().List(ctx, paginationOpts...)
	if err != nil {
		klog.Errorf("failed to list agents: %v", err)
		return nil, errors.ErrServerInternal
	}

	ts := make([]types.Agent, 0)
	for _, object := range objects {
		ts = append(ts, *model2Type(&object))
	}
	pageResult.Items = ts

	return pageResult, nil
}

// ── Agent job APIs (token auth) ──
func (a *agentController) getAuthAgent(ctx context.Context, agentToken string) (*model.Agent, error) {
	// 空 token 一律拒绝：db.WithToken("") 不追加 WHERE 条件（返回未过滤查询），
	// 若放行将因过滤条件缺失命中任意 agent 行（与请求中的 token 无从比对），造成未授权访问
	agentToken = strings.TrimSpace(agentToken)
	if agentToken == "" {
		return nil, errors.NewError(fmt.Errorf("invalid agent token"), http.StatusUnauthorized)
	}
	obj, err := a.factory.Agent().GetBy(ctx, db.WithToken(agentToken))
	if err != nil {
		klog.Errorf("failed to auth agent by token: %v", err)
		return nil, errors.NewError(fmt.Errorf("invalid agent token"), http.StatusUnauthorized)
	}
	if obj == nil {
		return nil, errors.NewError(fmt.Errorf("invalid agent token"), http.StatusUnauthorized)
	}
	klog.V(2).Infof("agent auth ok: id=%d name=%s", obj.Id, obj.Name)
	return obj, nil
}

func (a *agentController) Heartbeat(ctx context.Context, agentToken string, req *types.AgentHeartbeatRequest) error {
	obj, err := a.getAuthAgent(ctx, agentToken)
	if err != nil {
		return err
	}
	updates := map[string]interface{}{"status": model.AgentStatusOnline, "last_heartbeat": time.Now()}
	if req != nil {
		if req.Hostname != "" {
			updates["hostname"] = req.Hostname
		}
		if req.Version != "" {
			updates["version"] = req.Version
		}
	}
	return a.factory.Agent().InternalUpdate(ctx, obj.Id, updates)
}

func (a *agentController) Claim(ctx context.Context, agentToken string) (*types.Job, error) {
	// 只获取和自己相关的
	obj, err := a.getAuthAgent(ctx, agentToken)
	if err != nil {
		return nil, err
	}

	job, err := a.factory.Agent().Job().ClaimNext(ctx, obj.Id)
	if err != nil || job == nil {
		return nil, err
	}
	return &types.Job{
		Id:       job.Id,
		PlanId:   job.PlanId,
		AgentId:  job.AgentId,
		TaskName: job.TaskName,
		Kind:     job.Kind,
		Action:   job.Action,
		Image:    job.Image,
		Payload:  job.Payload,
		Status:   job.Status,
		Message:  job.Message,
	}, nil
}

func (a *agentController) AppendLogs(ctx context.Context, agentToken string, jobId int64, req *types.AgentJobLogsRequest) error {
	if _, err := a.getAuthAgent(ctx, agentToken); err != nil {
		return err
	}
	return a.factory.Agent().Job().AppendLogs(ctx, jobId, req.Chunk)
}

func (a *agentController) ReportResult(ctx context.Context, agentToken string, jobId int64, req *types.AgentJobResultRequest) error {
	if _, err := a.getAuthAgent(ctx, agentToken); err != nil {
		return err
	}
	status := model.JobSuccess
	if !req.Success {
		status = model.JobFailed
	}

	// 首个终态为准：服务端超时已判 failed 后，agent 的迟到上报不得把作业翻回 success
	job, err := a.factory.Agent().Job().Get(ctx, jobId)
	if err != nil {
		return err
	}
	if job != nil && (job.Status == model.JobSuccess || job.Status == model.JobFailed) {
		klog.Warningf("作业(%d)已是终态(%s)，忽略本次迟到上报(%s)", jobId, job.Status, status)
		return nil
	}

	return a.factory.Agent().Job().InternalUpdate(ctx, jobId, map[string]interface{}{
		"status": status, "message": req.Message, "result": req.Result,
	})
}

func (a *agentController) GetPlan(ctx context.Context, agentToken string, jobId int64) (*types.Plan, error) {
	obj, err := a.getAuthAgent(ctx, agentToken)
	if err != nil {
		return nil, err
	}
	job, err := a.factory.Agent().Job().Get(ctx, jobId)
	if err != nil || job == nil || job.AgentId != obj.Id {
		return nil, errors.NewError(fmt.Errorf("job not found"), http.StatusNotFound)
	}

	// 免用户态鉴权组装：agent 已通过 agent token 认证 + job 归属校验（上方 AgentId 比对）
	return plan.AssemblePlanView(ctx, a.factory, job.PlanId)
}

// ── helpers ──

func buildAgentFilters(uid int64, opt types.ListOptions, authorizedIDs []int64) []db.Options {
	var opts []db.Options
	if opt.NameSelector != "" {
		opts = append(opts, db.WithNameLike(opt.NameSelector))
	}
	opts = append(opts, db.WithUserOrResourceIDs(uid, authorizedIDs))
	if opt.AgentStatus != nil {
		opts = append(opts, db.WithStatus(*opt.AgentStatus))
	}
	return opts
}

func model2Type(o *model.Agent) *types.Agent {
	return &types.Agent{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:          o.Name,
		Type:          o.AgentType,
		UserID:        o.UserID,
		Status:        o.Status,
		Hostname:      o.Hostname,
		Version:       o.Version,
		LastHeartbeat: o.LastHeartbeat,
		Description:   o.Description,
		Token:         o.Token,
	}
}
