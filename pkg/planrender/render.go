/*
Copyright 2026 The Pixiu Authors.

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

package planrender

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util"
	pixiutpl "github.com/caoyingjunz/pixiu/template"
)

type Multinode struct {
	DockerMaster     []types.PlanNode
	DockerNode       []types.PlanNode
	ContainerdMaster []types.PlanNode
	ContainerdNode   []types.PlanNode
	StorageNode      []types.PlanNode
}

// Render 将 hosts / multinode / globals.yml / ssh key 渲染到 workDir/<planId>/。
func Render(workDir string, plan *types.Plan) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}
	// 主机名合法性校验：防畸形主机名注入 Ansible inventory（写库侧已校验，此处为渲染前兜底，先于任何写入）
	for _, node := range plan.Nodes {
		if !util.IsValidHostname(node.Name) {
			return fmt.Errorf("node %q has invalid hostname", node.Name)
		}
	}

	planDir := filepath.Join(workDir, fmt.Sprintf("%d", plan.Id))
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		return err
	}
	// 防路径穿越与符号链接写入：plan 目录解析真实路径后必须仍位于 workDir 之下（防目录被预植为软链）
	if err := ensureDirUnder(workDir, planDir); err != nil {
		return err
	}

	if err := writeTemplate(planDir, "hosts", pixiutpl.HostTemplate, plan); err != nil {
		return err
	}

	nodes, err := buildMultinode(plan, workDir)
	if err != nil {
		return err
	}
	if err = writeTemplate(planDir, "multinode", pixiutpl.MultiModeTemplate, nodes); err != nil {
		return err
	}

	// 启用自定义源时，将内容直接写入 pixiu 文件（容器内路径 /configs/pixiu）
	cfg := plan.Config
	if cfg.Component.CustomRepo != nil && cfg.Component.CustomRepo.Enable {
		pixiuFile := filepath.Join(planDir, "pixiu")
		if err = writeFileInDir(planDir, pixiuFile, []byte(cfg.Component.CustomRepo.Content)); err != nil {
			return fmt.Errorf("write custom repo file %s: %w", pixiuFile, err)
		}
	}

	return writeTemplate(planDir, "globals.yml", pixiutpl.GlobalsTemplate, &plan.Config)
}

// writeTemplate 渲染模板并写入 planDir 下的文件（经 writeFileInDir 统一加固）
func writeTemplate(planDir, name, text string, data interface{}) error {
	tpl := template.Must(template.New(name).Parse(text))
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return err
	}
	return writeFileInDir(planDir, filepath.Join(planDir, name), buf.Bytes())
}

// writeFileInDir 在 baseDir 内安全写文件：防路径穿越与符号链接写入，权限固定 0600。
func writeFileInDir(baseDir, path string, data []byte) error {
	// 防路径穿越：相对路径不得逃出 baseDir
	rel, err := filepath.Rel(baseDir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("refuse to write outside dir %s: %s", baseDir, path)
	}

	// O_NOFOLLOW：目标存在且为符号链接时直接失败，防写入被重定向到任意路径；
	// O_NONBLOCK：目标若为 FIFO 等特殊文件时不阻塞打开，随后由 Stat 检查拒绝
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0o600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return fmt.Errorf("refuse to write non-regular file: %s", path)
	}
	// 显式收紧权限：防既有文件的宽松权限被沿用
	if err = f.Chmod(0o600); err != nil {
		_ = f.Close()
		return err
	}
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// ensureDirUnder 解析真实路径后校验 dir 位于 root 之下（防中间目录为软链被引出信任根）
func ensureDirUnder(root, dir string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(realRoot, realDir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("dir %s escapes %s", dir, root)
	}
	return nil
}

func buildMultinode(plan *types.Plan, workDir string) (Multinode, error) {
	multinode := Multinode{
		DockerMaster:     make([]types.PlanNode, 0),
		DockerNode:       make([]types.PlanNode, 0),
		ContainerdMaster: make([]types.PlanNode, 0),
		ContainerdNode:   make([]types.PlanNode, 0),
		StorageNode:      make([]types.PlanNode, 0),
	}
	if plan == nil {
		return multinode, fmt.Errorf("plan is nil")
	}

	runtime := plan.Config.Runtime
	for _, node := range plan.Nodes {
		nodeAuth := node.Auth
		if _, err := writeRSA(plan.Id, node.Name, workDir, nodeAuth); err != nil {
			return multinode, err
		}

		if nodeAuth.Type == types.KeyAuth {
			if nodeAuth.Key == nil {
				return multinode, fmt.Errorf("node(%s) key auth config is empty", node.Name)
			}
			// 拷贝避免改写调用方数据
			key := *nodeAuth.Key
			key.File = fmt.Sprintf("/configs/ssh/%s/id_rsa", node.Name)
			nodeAuth.Key = &key
		}
		if nodeAuth.Type == types.PasswordAuth && nodeAuth.Password == nil {
			return multinode, fmt.Errorf("node(%s) password auth config is empty", node.Name)
		}
		planNode := types.PlanNode{Name: node.Name, Auth: nodeAuth}
		roles := node.Role
		if len(roles) == 0 {
			continue
		}

		if runtime.IsDocker() {
			for _, role := range roles {
				if role == model.MasterRole {
					multinode.DockerMaster = append(multinode.DockerMaster, planNode)
				}
				if role == model.NodeRole {
					multinode.DockerNode = append(multinode.DockerNode, planNode)
				}
			}
		}
		if runtime.IsContainerd() {
			for _, role := range roles {
				if role == model.MasterRole {
					multinode.ContainerdMaster = append(multinode.ContainerdMaster, planNode)
				}
				if role == model.NodeRole {
					multinode.ContainerdNode = append(multinode.ContainerdNode, planNode)
				}
			}
		}
		for _, role := range roles {
			if role == model.StorageRole {
				multinode.StorageNode = append(multinode.StorageNode, planNode)
			}
		}
	}

	return multinode, nil
}

func writeRSA(planId int64, name, workDir string, auth types.PlanNodeAuth) (string, error) {
	if auth.Type != types.KeyAuth {
		return "", nil
	}
	if auth.Key == nil {
		return "", fmt.Errorf("node(%s) key auth config is empty", name)
	}
	// 主机名合法性校验：name 参与目录路径构造，防路径穿越与畸形目录名
	if !util.IsValidHostname(name) {
		return "", fmt.Errorf("node %q has invalid hostname", name)
	}

	planDir := filepath.Join(workDir, fmt.Sprintf("%d", planId))
	rsaDir := filepath.Join(planDir, "ssh", name)
	if err := os.MkdirAll(rsaDir, 0o755); err != nil {
		return "", err
	}
	// 防符号链接：ssh 或 <name> 目录被预植为软链时，rsaDir 解析后必须仍位于 planDir 之下
	if err := ensureDirUnder(planDir, rsaDir); err != nil {
		return "", err
	}

	f := filepath.Join(rsaDir, "id_rsa")
	if err := writeFileInDir(planDir, f, []byte(auth.Key.Data)); err != nil {
		return "", err
	}
	return f, nil
}

// NewPlan 将 DB 模型转为 types.Plan，供 local 模式控制面渲染使用。
func NewPlan(planId int64, cfg *model.Config, nodes []model.Node) (*types.Plan, error) {
	plan := &types.Plan{
		PixiuMeta: types.PixiuMeta{Id: planId},
		Nodes:     make([]types.PlanNode, 0, len(nodes)),
	}
	if cfg != nil {
		ks := types.KubernetesSpec{}
		if err := ks.Unmarshal(cfg.Kubernetes); err != nil {
			return nil, err
		}
		ns := types.NetworkSpec{}
		if err := ns.Unmarshal(cfg.Network); err != nil {
			return nil, err
		}
		rs := types.RuntimeSpec{}
		if err := rs.Unmarshal(cfg.Runtime); err != nil {
			return nil, err
		}
		cs := types.ComponentSpec{}
		if err := cs.Unmarshal(cfg.Component); err != nil {
			return nil, err
		}
		plan.Config = types.PlanConfig{
			PlanId:     planId,
			Region:     cfg.Region,
			OSImage:    cfg.OSImage,
			Kubernetes: ks,
			Network:    ns,
			Runtime:    rs,
			Component:  cs,
		}
	}
	for _, n := range nodes {
		auth := types.PlanNodeAuth{}
		if err := auth.Unmarshal(n.Auth); err != nil {
			return nil, err
		}
		var roles []string
		if n.Role != "" {
			roles = strings.Split(n.Role, ",")
		}
		plan.Nodes = append(plan.Nodes, types.PlanNode{
			Name:   n.Name,
			PlanId: planId,
			Role:   roles,
			CRI:    n.CRI,
			Ip:     n.Ip,
			Auth:   auth,
		})
	}
	return plan, nil
}
