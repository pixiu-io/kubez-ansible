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

package node

import (
	"context"
	"strings"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	pixiuutil "github.com/caoyingjunz/pixiu/pkg/util"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
	sshutil "github.com/caoyingjunz/pixiu/pkg/util/ssh"
)

type NodeGetter interface {
	Node() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateNodeRequest) error
	Update(ctx context.Context, nodeId int64, req *types.UpdateNodeRequest) error
	Delete(ctx context.Context, nodeId int64) error
	Get(ctx context.Context, nodeId int64) (*types.NodeResult, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)

	CheckConnectivity(ctx context.Context, req *types.NodeConnectivityRequest) (*types.NodeConnectivityResult, error)
}

type nodeController struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func NewNode(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &nodeController{
		cc:      cfg,
		factory: f,
	}
}

func (n *nodeController) Create(ctx context.Context, req *types.CreateNodeRequest) error {
	// 主机名格式校验：与前端主机页保持一致（trim 后校验）
	name := strings.TrimSpace(req.Name)
	if !pixiuutil.IsValidHostname(name) {
		klog.Errorf("invalid node name %q", req.Name)
		return errors.ErrInvalidNodeName
	}
	req.Name = name

	if err := n.preCreate(ctx, req); err != nil {
		klog.Errorf("pre-create check failed for node %s: %v", req.Name, err)
		return err
	}

	authStr, err := req.Auth.Marshal()
	if err != nil {
		klog.Errorf("marshal node auth: %v", err)
		return errors.ErrInvalidRequest
	}
	object := &model.Node{
		Name:   req.Name,
		UserId: req.UserId,
		PlanId: req.PlanId,
		Ip:     req.Ip,
		Auth:   authStr,
	}
	if _, err = n.factory.Plan().Node().Create(ctx, nil, object); err != nil {
		klog.Errorf("failed to create node %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}

	return nil
}

// 创建前置检查：ip 全局唯一，不允许与已存在节点冲突
func (n *nodeController) preCreate(ctx context.Context, req *types.CreateNodeRequest) error {
	_, err := n.factory.Plan().Node().GetByIP(ctx, req.Ip)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return nil
		}
		klog.Errorf("get node by ip %s: %v", req.Ip, err)
		return errors.ErrServerInternal
	}
	return errors.ErrNodeIPExists
}

// 更新前置检查：资源存在 + 非超级管理员只能更新自己的节点
func (n *nodeController) preUpdate(ctx context.Context, nodeId int64) error {
	object, err := n.factory.Plan().Node().Get(ctx, nodeId)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return errors.ErrNodeNotFound
		}
		klog.Errorf("get node %d: %v", nodeId, err)
		return errors.ErrServerInternal
	}
	if err = util.CheckResourceAccess(ctx, n.factory, object.UserId, types.ResourceTypeNode, nodeId); err != nil {
		return err
	}
	return nil
}

func (n *nodeController) Update(ctx context.Context, nodeId int64, req *types.UpdateNodeRequest) error {
	if err := n.preUpdate(ctx, nodeId); err != nil {
		klog.Errorf("pre-update check failed for node(%d): %v", nodeId, err)
		return err
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		// 主机名格式校验：与前端主机页保持一致（trim 后校验）
		name := strings.TrimSpace(*req.Name)
		if !pixiuutil.IsValidHostname(name) {
			klog.Errorf("invalid node name %q", *req.Name)
			return errors.ErrInvalidNodeName
		}
		updates["name"] = name
	}
	if req.Ip != nil {
		updates["ip"] = *req.Ip
	}
	if req.Auth != nil {
		authStr, e := req.Auth.Marshal()
		if e != nil {
			klog.Errorf("marshal auth: %v", e)
			return errors.ErrInvalidRequest
		}
		updates["auth"] = authStr
	}
	if len(updates) == 0 {
		return nil
	}

	if err := n.factory.Plan().Node().Update(ctx, nil, nodeId, req.ResourceVersion, updates); err != nil {
		if err == utilerrors.ErrRecordNotFound {
			return errors.ErrNodeNotFound
		}
		klog.Errorf("update node %d: %v", nodeId, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (n *nodeController) Delete(ctx context.Context, nodeId int64) error {
	// 非超级管理员只能删除自己的节点
	object, err := n.factory.Plan().Node().Get(ctx, nodeId)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return errors.ErrNodeNotFound
		}
		klog.Errorf("get node %d: %v", nodeId, err)
		return errors.ErrServerInternal
	}
	if err := util.CheckResourceAccess(ctx, n.factory, object.UserId, types.ResourceTypeNode, nodeId); err != nil {
		return err
	}

	if _, err := n.factory.Plan().Node().Delete(ctx, nodeId); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return errors.ErrNodeNotFound
		}
		klog.Errorf("delete node %d: %v", nodeId, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (n *nodeController) Get(ctx context.Context, nodeId int64) (*types.NodeResult, error) {
	object, err := n.factory.Plan().Node().Get(ctx, nodeId)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return nil, errors.ErrNodeNotFound
		}
		klog.Errorf("get node %d: %v", nodeId, err)
		return nil, errors.ErrServerInternal
	}

	// 非超级管理员只能查看自己的节点或被 scope 授权的节点
	if err = util.CheckResourceAccess(ctx, n.factory, object.UserId, types.ResourceTypeNode, nodeId); err != nil {
		return nil, err
	}

	return model2Node(object), nil
}

// canProbeWithoutNode 免库内节点（node_id<=0，用户直接传 host+凭据）的连通性检测仅超级管理员可用：
// 防普通登录用户将其作为零痕迹的内网 SSH 探测 / 凭据验证跳板。
func canProbeWithoutNode(user *model.User) bool {
	return user != nil && user.Role == model.RoleRoot
}

// CheckConnectivity 节点 SSH 连通性检测：node_id>0 走库内认证（需 owner 校验），否则用用户传入的 host+凭据直接检测（仅超管）
func (n *nodeController) CheckConnectivity(ctx context.Context, req *types.NodeConnectivityRequest) (*types.NodeConnectivityResult, error) {
	var (
		webssh = &types.WebSSHRequest{}
		err    error
	)
	if req.NodeId > 0 {
		// 模式A：从库取认证
		object, e := n.factory.Plan().Node().Get(ctx, req.NodeId)
		if e != nil {
			if utilerrors.IsRecordNotFound(e) {
				return nil, errors.ErrNodeNotFound
			}
			klog.Errorf("get node %d: %v", req.NodeId, e)
			return nil, errors.ErrServerInternal
		}
		if e := util.CheckResourceAccess(ctx, n.factory, object.UserId, types.ResourceTypeNode, req.NodeId); e != nil {
			return nil, e
		}
		var auth types.PlanNodeAuth
		if e := auth.Unmarshal(object.Auth); e != nil {
			return nil, errors.ErrInvalidRequest
		}
		if webssh, err = sshutil.ResolveAuth(&auth); err != nil {
			return nil, errors.ErrInvalidRequest
		}
		webssh.Host = object.Ip
	} else {
		// 模式B：用户直接传凭据（不落库）——仅超级管理员可用（普通用户请先保存节点走模式A）
		if user, e := httputils.GetUserFromContext(ctx); e != nil || !canProbeWithoutNode(user) {
			return nil, errors.ErrConnectivityAdminOnly
		}
		if req.Host == "" {
			return nil, errors.ErrInvalidRequest
		}
		webssh.Host = req.Host
		webssh.Port = req.Port
		webssh.User = req.User
		if webssh.User == "" {
			webssh.User = "root"
		}
		webssh.Password = req.Password
		webssh.PrivateKey = req.PrivateKey
	}

	result := &types.NodeConnectivityResult{Host: webssh.Host, Port: webssh.Port, User: webssh.User}
	if webssh.Port == 0 {
		webssh.Port = 22
		result.Port = 22
	}
	client, e := sshutil.NewSSHClient(webssh)
	if e != nil {
		// 连通性失败返回 200 + connected=false + message，不将业务失败当 HTTP 错误
		result.Message = e.Error()
		return result, nil
	}
	defer client.Close()
	result.Connected = true
	return result, nil
}

func (n *nodeController) List(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	// 资源级授权：非超管用户叠加 scope 授权的 node id（超管走 listOption.UserId 现状即可）
	authorizedNodeIDs, err := util.AuthorizedResourceIDs(ctx, n.factory, types.ResourceTypeNode)
	if err != nil {
		return nil, err
	}

	opts := []db.Options{
		db.WithUserOrResourceIDs(listOption.UserId, authorizedNodeIDs),
		db.WithNameLike(listOption.NameSelector),
	}
	if listOption.PlanId != nil {
		opts = append(opts, db.WithPlanIdEq(*listOption.PlanId))
	}

	pageResult.Total, err = n.factory.Plan().Node().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("count nodes: %v", err)
		pageResult.Message = err.Error()
		return nil, errors.ErrServerInternal
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, []db.Options{
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	}...)

	objects, err := n.factory.Plan().Node().List(ctx, opts...)
	if err != nil {
		klog.Errorf("list nodes: %v", err)
		pageResult.Message = err.Error()
		return nil, errors.ErrServerInternal
	}

	items := make([]types.NodeResult, 0)
	for i := range objects {
		items = append(items, *model2Node(&objects[i]))
	}
	// 填充所属集群名称（非核心数据，失败仅记录日志，不阻塞列表）
	if err := n.fillClusterNames(ctx, items); err != nil {
		klog.Errorf("fill node cluster names: %v", err)
	}

	pageResult.Items = items
	return pageResult, nil
}

// fillClusterNames 按 plan_id 批量填充所属集群名称：
// 优先自建集群 alias_name（plan 注册集群时 AliasName 即 plan 名称），无集群记录时回退 plan 名称。
func (n *nodeController) fillClusterNames(ctx context.Context, items []types.NodeResult) error {
	planIDs := make([]int64, 0, len(items))
	seen := make(map[int64]struct{}, len(items))
	for i := range items {
		pid := items[i].PlanId
		if pid == 0 {
			continue
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		planIDs = append(planIDs, pid)
	}
	if len(planIDs) == 0 {
		return nil
	}

	clusters, err := n.factory.Cluster().List(ctx, db.WithPlanIn(planIDs))
	if err != nil {
		return err
	}
	clusterNames := make(map[int64]string, len(clusters))
	for i := range clusters {
		name := clusters[i].AliasName
		if name == "" {
			name = clusters[i].Name
		}
		clusterNames[clusters[i].PlanId] = name
	}

	// 未注册集群的 plan，回退显示 plan 名称
	fallbackIDs := make([]int64, 0, len(planIDs))
	for _, pid := range planIDs {
		if _, ok := clusterNames[pid]; !ok {
			fallbackIDs = append(fallbackIDs, pid)
		}
	}
	if len(fallbackIDs) > 0 {
		plans, err := n.factory.Plan().List(ctx, db.WithIdIn(fallbackIDs))
		if err != nil {
			return err
		}
		for i := range plans {
			clusterNames[plans[i].Id] = plans[i].Name
		}
	}

	for i := range items {
		items[i].ClusterName = clusterNames[items[i].PlanId]
	}
	return nil
}

func model2Node(o *model.Node) *types.NodeResult {
	auth := types.NodeAuthResult{}
	if o.Auth != "" {
		var spec types.PlanNodeAuth
		if err := spec.Unmarshal(o.Auth); err == nil {
			auth = types.NodeAuthResult{Type: spec.Type, Port: spec.Port}
		}
	}
	return &types.NodeResult{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:   o.Name,
		UserId: o.UserId,
		Ip:     o.Ip,
		Auth:   auth,
		PlanId: o.PlanId,
	}
}
