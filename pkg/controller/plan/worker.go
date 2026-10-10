/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (phe "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plan

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	pixiuutil "github.com/caoyingjunz/pixiu/pkg/util"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Handler interface {
	GetPlanId() int64
	GetAction() string // kubez-ansible {实际执行命名，比如 deploy，}

	Name() string         // 检查项名称
	Step() model.PlanStep // 未开始，运行中，异常和完成
	Run() error           // 执行
}

type handlerTask struct {
	data TaskData
}

func (t handlerTask) GetPlanId() int64     { return t.data.PlanId }
func (t handlerTask) GetAction() string    { return "" }
func (t handlerTask) Step() model.PlanStep { return model.RunningPlanStep }

func newHandlerTask(data TaskData) handlerTask {
	return handlerTask{data: data}
}

func (p *plan) Run(ctx context.Context, workers int) error {
	// 进程启动时，尝试同步任务状态
	klog.Infof("starting to sync task manager")
	if err := p.SyncTaskStatus(ctx); err != nil {
		return err
	}

	// 启动部署计划控制器
	klog.Infof("starting deployment manager")
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, p.worker, time.Second)
	}
	return nil
}

func (p *plan) worker(ctx context.Context) {
	for p.process(ctx) {
	}
}

func (p *plan) process(ctx context.Context) bool {
	key, quit := taskQueue.Get()
	if quit {
		return false
	}
	defer taskQueue.Done(key)

	p.syncHandler(ctx, key.(int64))
	return true
}

type TaskData struct {
	PlanId int64
	Plan   *model.Plan   // 部署属性
	Config *model.Config // 部署配置
	Nodes  []model.Node  // 部署节点
}

var (
	// sshUserRe SSH 登录用户：Linux 用户名规范（小写字母/下划线开头，后接小写字母/数字/下划线/中划线）
	sshUserRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	// k8sVersionRe kubernetes 版本号：v?x.y 或 v?x.y.z（宽松匹配，避免误杀已有计划）
	k8sVersionRe = regexp.MustCompile(`^v?\d+(\.\d+){1,2}$`)
	// authUnsafeRe 注入面字符：inventory 中 ansible_ssh_pass/user 为无引号渲染，空白或引号会破坏行结构
	authUnsafeRe = regexp.MustCompile(`[\s"']`)
)

// validate 部署预检查（部署流程第 1 步）：拦截畸形/恶意计划数据，防御纵深。
// 纯函数，不依赖外部 IO；原则：宁可漏检，不可误杀已有合法计划（只校验格式定义明确的字段）。
func (t TaskData) validate() error {
	if t.PlanId <= 0 || t.Plan == nil {
		return fmt.Errorf("部署计划数据异常，请重新创建部署计划")
	}
	if t.Config == nil {
		return fmt.Errorf("部署计划配置缺失，请先完善部署配置")
	}
	if len(t.Nodes) == 0 {
		return fmt.Errorf("部署计划未包含任何节点，请先添加节点")
	}

	for i := range t.Nodes {
		if err := validateNode(&t.Nodes[i]); err != nil {
			return err
		}
	}
	return validateConfig(t.Config)
}

// validateNode 校验单个部署节点的关键字段与注入面
func validateNode(n *model.Node) error {
	// 主机名：防畸形主机名注入 Ansible inventory 节点行（写库侧已校验，此处为部署前兜底）
	if !pixiuutil.IsValidHostname(strings.TrimSpace(n.Name)) {
		return fmt.Errorf("节点主机名 %q 不合法（1-63 位，小写字母/数字/中划线，且不能以中划线开头或结尾），请修改后重试", n.Name)
	}
	if net.ParseIP(n.Ip) == nil {
		return fmt.Errorf("节点 %s 的 IP 地址 %q 不合法，请修改后重试", n.Name, n.Ip)
	}

	var auth types.PlanNodeAuth
	if err := auth.Unmarshal(n.Auth); err != nil {
		return fmt.Errorf("节点 %s 的认证信息无法解析，请重新配置", n.Name)
	}
	if auth.Port < 0 || auth.Port > 65535 {
		return fmt.Errorf("节点 %s 的 SSH 端口 %d 不合法（有效范围 1-65535）", n.Name, auth.Port)
	}

	switch auth.Type {
	case types.PasswordAuth:
		if auth.Password == nil || auth.Password.User == "" || auth.Password.Password == "" {
			return fmt.Errorf("节点 %s 缺少密码认证信息（用户/密码），请重新配置", n.Name)
		}
		if !sshUserRe.MatchString(auth.Password.User) {
			return fmt.Errorf("节点 %s 的 SSH 用户 %q 不合法（仅支持小写字母/数字/下划线/中划线），请修改后重试", n.Name, auth.Password.User)
		}
		if authUnsafeRe.MatchString(auth.Password.Password) {
			return fmt.Errorf("节点 %s 的密码包含不支持字符（空白/引号），请更换密码", n.Name)
		}
	case types.KeyAuth:
		if auth.Key == nil || auth.Key.Data == "" {
			return fmt.Errorf("节点 %s 缺少密钥认证信息（私钥内容），请重新配置", n.Name)
		}
	default:
		return fmt.Errorf("节点 %s 的认证方式 %q 不支持（仅支持 password/key），请重新配置", n.Name, auth.Type)
	}
	return nil
}

// validateConfig 校验部署配置中格式定义明确的字段（k8s 版本 / 集群 CIDR）
func validateConfig(cfg *model.Config) error {
	ks := types.KubernetesSpec{}
	if err := ks.Unmarshal(cfg.Kubernetes); err != nil {
		return fmt.Errorf("部署配置（kubernetes）数据异常，请重新保存部署配置")
	}
	if !k8sVersionRe.MatchString(strings.TrimSpace(ks.KubernetesVersion)) {
		return fmt.Errorf("K8s 版本 %q 不合法（形如 v1.28.0），请修改后重试", ks.KubernetesVersion)
	}

	ns := types.NetworkSpec{}
	if err := ns.Unmarshal(cfg.Network); err != nil {
		return fmt.Errorf("部署配置（network）数据异常，请重新保存部署配置")
	}
	if ns.PodNetwork != "" {
		if _, _, err := net.ParseCIDR(ns.PodNetwork); err != nil {
			return fmt.Errorf("容器子网 %q 不是合法的 CIDR（形如 10.244.0.0/16），请修改后重试", ns.PodNetwork)
		}
	}
	if ns.ServiceNetwork != "" {
		if _, _, err := net.ParseCIDR(ns.ServiceNetwork); err != nil {
			return fmt.Errorf("Service IP 段 %q 不是合法的 CIDR，请修改后重试", ns.ServiceNetwork)
		}
	}
	return nil
}

func (p *plan) getTaskData(ctx context.Context, planId int64) (TaskData, error) {
	pp, err := p.factory.Plan().Get(ctx, planId)
	if err != nil {
		return TaskData{}, err
	}
	nodes, err := p.factory.Plan().Node().List(ctx, db.WithPlanIdEq(planId))
	if err != nil {
		return TaskData{}, err
	}
	cfg, err := p.factory.Plan().Config().GetByPlan(ctx, planId)
	if err != nil {
		return TaskData{}, err
	}

	return TaskData{
		PlanId: planId,
		Plan:   pp,
		Config: cfg,
		Nodes:  nodes,
	}, nil
}

// 实际处理函数
// 处理步骤:
// 0. 更新集群状态为部署中
// 1. 检查部署参数是否符合要求
// 2. 渲染环境
// 3. 执行部署
// 4. 部署后环境清理
func (p *plan) syncHandler(ctx context.Context, planId int64) {
	klog.Infof("starting plan(%d) task", planId)
	defer klog.Infof("completed plan(%d) task", planId)

	// 尝试去更新集群状态为部署中
	if err := p.factory.Cluster().UpdateByPlan(ctx, planId, map[string]interface{}{"status": model.ClusterStatusDeploy}); err != nil {
		klog.Warningf("failed to update cluster status to %v, ignoring: %v", model.ClusterStatusDeploy, err)
	}

	taskData, err := p.getTaskData(ctx, planId)
	if err != nil {
		klog.Errorf("failed to get task data: %v", err)
		return
	}
	runner, err := p.GetRunner(ctx, taskData.Config.OSImage)
	if err != nil {
		klog.Errorf("failed to get image(%s) for worker: %v", taskData.Config.OSImage, err)
		return
	}
	task := newHandlerTask(taskData)

	var handlers []Handler
	if taskData.Plan.ExecMode == model.PlanExecModeAgent {
		handlers = p.buildAgentHandlers(task, runner, taskData)
	} else {
		handlers = p.buildLocalHandlers(task, runner)
	}

	status := model.ClusterStatusRunning
	if err = p.syncTasks(handlers...); err != nil {
		status = model.ClusterStatusFailed
		klog.Warningf("failed to sync task: %v", err)
	}

	if err = p.factory.Cluster().UpdateByPlan(ctx, planId, map[string]interface{}{"status": status}); err != nil {
		klog.Errorf("failed to update cluster status to %d: %v", status, err)
	}
}

func (p *plan) buildLocalHandlers(task handlerTask, runner string) []Handler {
	// Runner的工作目录
	dir := p.WorkDir()
	// 部署容器等待超时，来自 worker.deploy_timeout 配置(秒)
	waitTimeout := time.Duration(p.cc.Worker.DeployTimeout) * time.Second
	return []Handler{
		Runner{handlerTask: task, image: runner, factory: p.factory},
		Render{handlerTask: task, dir: dir},
		Check{handlerTask: task},
		BootStrap{handlerTask: task, dir: dir, runner: runner, waitTimeout: waitTimeout},
		DeployMaster{handlerTask: task, dir: dir, runner: runner, waitTimeout: waitTimeout},
		DeployNode{handlerTask: task, dir: dir, runner: runner, waitTimeout: waitTimeout},
		DeployChart{handlerTask: task, dir: dir, runner: runner, waitTimeout: waitTimeout},
		Register{handlerTask: task, factory: p.factory},
	}
}

func (p *plan) buildAgentHandlers(task handlerTask, runner string, data TaskData) []Handler {
	agentId := data.Plan.DeployAgentId
	reg := Register{handlerTask: task, factory: p.factory}
	payload, _ := buildRegisterPayload(data.Nodes)

	return []Handler{
		Check{handlerTask: task},
		// 镜像拉取、配置渲染与部署均在边缘 Agent 执行；控制面只下发 Job 并等待结果
		AgentJob{
			handlerTask: task, factory: p.factory, agentId: agentId,
			stepName: "前置准备", step: model.RunningPlanStep,
			kind: model.JobPullImage, action: "pull_image", image: runner,
			timeout: 30 * time.Minute,
		},
		AgentJob{
			handlerTask: task, factory: p.factory, agentId: agentId,
			stepName: "配置渲染", step: model.RunningPlanStep,
			kind: model.JobRenderConfig, action: "render", image: runner,
			timeout: 10 * time.Minute,
		},
		AgentJob{
			handlerTask: task, factory: p.factory, agentId: agentId,
			stepName: "初始化部署环境", step: model.RunningPlanStep,
			kind: model.JobRunContainer, action: "bootstrap-servers", image: runner,
			timeout: 30 * time.Minute,
		},
		AgentJob{
			handlerTask: task, factory: p.factory, agentId: agentId,
			stepName: "部署Master", step: model.RunningPlanStep,
			kind: model.JobRunContainer, action: "deploy-master", image: runner,
			timeout: 60 * time.Minute,
		},
		AgentJob{
			handlerTask: task, factory: p.factory, agentId: agentId,
			stepName: "部署Node", step: model.RunningPlanStep,
			kind: model.JobRunContainer, action: "deploy-node", image: runner,
			timeout: 60 * time.Minute,
		},
		AgentJob{
			handlerTask: task, factory: p.factory, agentId: agentId,
			stepName: "部署基础组件", step: model.RunningPlanStep,
			kind: model.JobRunContainer, action: "apply", image: runner,
			timeout: 60 * time.Minute,
		},
		AgentJob{
			handlerTask: task, factory: p.factory, agentId: agentId,
			stepName: "集群注册", step: model.CompletedPlanStep,
			kind: model.JobFetchKubeconfig, action: "register", payload: payload,
			timeout: 15 * time.Minute,
			onSuccess: func(result string) error {
				return reg.finishWithKubeConfig(result)
			},
		},
	}
}

func (p *plan) createPlanTasksIfNotExist(tasks ...Handler) error {
	for _, task := range tasks {
		planId := task.GetPlanId()
		name := task.Name()
		step := task.Step()

		_, err := p.factory.Plan().Task().GetByName(context.TODO(), planId, name)
		// 存在则直接返回
		if err == nil {
			return nil
		}
		// 非不存在报错则报异常
		if !errors.IsRecordNotFound(err) {
			klog.Infof("failed to get plan(%d) tasks(%s) for first created: %v", planId, name, err)
			return err
		}

		// 不存在记录则新建
		if _, err = p.factory.Plan().Task().Create(context.TODO(), &model.Task{
			Name:   name,
			PlanId: planId,
			Step:   step,
			Action: task.GetAction(),
			Status: model.UnStartPlanStatus,
		}); err != nil {
			klog.Errorf("failed to init plan(%d) task(%s): %v", planId, name, err)
			return err
		}
	}

	return nil
}

func (p *plan) WorkDir() string {
	return p.cc.Worker.WorkDir
}

func (p *plan) GetRunner(ctx context.Context, osImage string) (string, error) {
	dist, err := p.factory.Distribution().GetDistributionByName(ctx, osImage)
	if err != nil {
		return "", err
	}
	if dist == nil || dist.Runner == "" {
		return "", fmt.Errorf("osImage(%s) runner not found", osImage)
	}

	obj, err := p.factory.Runner().GetBy(ctx, db.WithName(dist.Runner))
	if err != nil {
		return "", err
	}

	return obj.EngineImage, nil
}

// 同步任务状态
// 任务启动时设置为运行中，结束时同步为结束状态(成功或者失败)
// TODO: 后续优化，判断对应部署容器是否在运行，根据容器的运行结果同步状态
func (p *plan) syncStatus(ctx context.Context, planId int64) error {
	tasks, err := p.factory.Plan().Task().List(ctx, planId)
	if err != nil {
		return err
	}

	updated := false
	for _, task := range tasks {
		if task.Status != model.RunningPlanStatus {
			continue
		}
		if _, err = p.factory.Plan().Task().Update(ctx, planId, task.Name, map[string]interface{}{
			"status": model.FailedPlanStatus, "step": model.FailedPlanStep, "message": "服务异常修正，请重新启动部署计划", "gmt_modified": time.Now(),
		}); err != nil {
			klog.Errorf("failed to update plan(%d) status: %v", planId, err)
			return err
		}
		updated = true
	}
	if updated {
		if err := p.updateStatus(ctx, planId, model.FailedPlanStatus); err != nil {
			klog.Errorf("failed to sync plan(%d) status after correcting running tasks: %v", planId, err)
			return err
		}
	}
	return nil
}

// updateStatus 部署状态机状态回写：worker 并发场景无 resource_version，走 UpdateBy 绕过乐观锁。
// 后续如需校验非法状态迁移（如「运行中」→「未开始」），在此处扩展。
func (p *plan) updateStatus(ctx context.Context, planId int64, status model.TaskStatus) error {
	return p.factory.Plan().UpdateBy(ctx, map[string]interface{}{"status": status}, db.WithId(planId))
}

func (p *plan) syncTasks(tasks ...Handler) error {
	// 初始化记录
	if err := p.createPlanTasksIfNotExist(tasks...); err != nil {
		return err
	}
	if len(tasks) == 0 {
		return nil
	}

	// 重置 plan 状态为进行中
	planId := tasks[0].GetPlanId()
	if err := p.updateStatus(context.TODO(), planId, model.RunningPlanStatus); err != nil {
		klog.Errorf("failed to sync plan(%d) status to running: %v", planId, err)
		return err
	}

	// 执行任务并更新状态
	for i, task := range tasks {
		planId := task.GetPlanId()
		name := task.Name()
		klog.Infof("starting plan(%d) task(%s)", planId, name)

		// TODO: 通过闭包方式优化
		start, err := p.factory.Plan().Task().Update(context.TODO(), planId, name, map[string]interface{}{
			"status": model.RunningPlanStatus, "message": "", "gmt_create": time.Now(),
		})
		if err != nil {
			klog.Errorf("failed to update plan(%d) status before run task(%s): %v", planId, name, err)
			return err
		}
		taskC.SetByTask(planId, *start)

		status := model.SuccessPlanStatus
		step := task.Step()
		message := "执行成功"

		// 执行检查
		runErr := task.Run()
		if runErr != nil {
			status = model.FailedPlanStatus
			step = model.FailedPlanStep
			message = runErr.Error()
		}

		// 执行完成之后更新状态
		end, err := p.factory.Plan().Task().Update(context.TODO(), planId, name, map[string]interface{}{
			"status": status, "message": message, "step": step, "gmt_modified": time.Now(),
		})
		if err != nil {
			klog.Errorf("failed to update plan(%d) status after run task(%s): %v", planId, name, err)
			return err
		}

		// 中间步骤成功保持「运行中」；失败或最后一步成功再回写 plan.status
		if status == model.FailedPlanStatus || (status == model.SuccessPlanStatus && i == len(tasks)-1) {
			if err = p.updateStatus(context.TODO(), planId, status); err != nil {
				klog.Errorf("failed to sync plan(%d) status after task(%s): %v", planId, name, err)
				return err
			}
		}
		taskC.SetByTask(planId, *end)
		if runErr != nil {
			klog.Errorf("run plan(%d) task(%s) failed: %v", planId, name, runErr)
			return runErr
		}
		klog.Infof("completed plan(%d) task(%s)", planId, name)
	}

	return nil
}
