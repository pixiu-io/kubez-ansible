# Add Node 需求说明

## 1. 目标

新增 `add-node` 子命令，用于向已有 Kubernetes 集群添加 Worker Node。

该命令应复用现有 `deploy-node.yml` 的节点准备和加入流程，仅替换集群加入参数的来源：

- `deploy-node`：从 master 获取 token、CA hash 等参数。
- `add-node`：从 `globals.yml` 获取用户维护的 token 和 CA hash。

除加入参数来源外，两个流程应保持一致。

## 2. 命令

```bash
kubez-ansible \
  -i multinode \
  --limit kube03 \
  --configdir /etc/kubez \
  add-node
```

参数说明：

- `-i, --inventory`：指定 Ansible inventory。
- `--limit`：指定要加入集群的 Node。
- `--configdir`：指定包含 `globals.yml` 的配置目录。
- `add-node`：执行 Node 加入流程。

不新增单独的 join 配置文件，也不新增 `--node-config` 参数。

## 3. 配置项

### 3.1 默认变量

在 `ansible/group_vars/all.yml` 中定义变量和默认值：

```yaml
add_node_token: ""
add_node_token_ca_cert_hash: ""
```

### 3.2 环境配置

用户在 `etc/kubez/globals.yml` 或通过 `--configdir` 指定的 `globals.yml` 中维护实际值：

```yaml
add_node_token: "abcdef.0123456789abcdef"
add_node_token_ca_cert_hash: "sha256:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
```

认证参数属于敏感信息，不应在任务输出和日志中完整打印。

## 4. API Server 地址

`add_node_api_server` 不由用户重复配置，应自动计算：

1. `kube_vip_address` 已配置时，使用该地址。
2. 未配置 VIP 时，使用 `kube-master[0]` 的 `api_interface_address`。
3. 端口使用 `kube_vip_port`，默认值为 `6443`。

最终地址格式为：

```text
<api-server-address>:<port>
```

自动获取地址不能执行 master 上的 `kubeadm`、`kubectl` 或认证文件读取操作；只允许使用 inventory 和已采集的主机事实信息。

## 5. 执行流程

`add-node` 应使用独立入口 `ansible/add-node.yml`，但任务内容应与 `ansible/roles/kubernetes/tasks/deploy-node.yml` 保持一致：

1. 拉取 Kubernetes 基础镜像。
2. 加载内核模块。
3. 配置 Kubernetes 所需的 sysctl 参数。
4. 关闭 swap 并更新 `/etc/fstab`。
5. 检测目标 Node 的 CRI socket。
6. 校验 token 和 CA hash 配置。
7. 使用自动计算的 API Server 地址和配置中的认证参数执行 `kubeadm join`。
8. 保留现有的 Node 污点处理逻辑。

Node 加入命令的参数等价于：

```bash
kubeadm join <api-server>:<port> \
  --token <token> \
  --discovery-token-ca-cert-hash <hash> \
  --cri-socket <socket>
```

## 6. 约束

- 第一阶段只支持 Worker Node。
- 不读取 `/etc/kubernetes/admin.conf`。
- 不在 master 上创建或刷新 token。
- 不在 master 上获取 CA hash。
- 不自动生成或上传 join 配置文件。
- 不改变现有 `deploy` 和 `deploy-node` 行为。
- 必须支持通过 `--limit` 精确指定目标 Node。
- 目标节点应已完成基础系统依赖安装，或通过现有公共流程完成准备。

## 7. 校验要求

执行前必须校验：

- `add_node_token` 非空。
- `add_node_token_ca_cert_hash` 非空。
- API Server 地址能够从 VIP 或 master inventory 信息计算得到。
- 目标 Node 位于 `kube-node` 组。
- 目标 Node 未加入其他 Kubernetes 集群，或明确提示用户先执行 reset。

## 8. 验收标准

- `kubez-ansible add-node --help` 能显示该子命令。
- 不提供 token 或 CA hash 时，流程在修改目标节点前失败。
- 使用 `--limit kube03` 时，只在 `kube03` 执行加入操作。
- API Server 地址不需要在 `globals.yml` 中重复维护。
- `kubeadm join` 使用 `globals.yml` 中的 token 和 CA hash。
- 执行过程中不运行 master 上的 token 创建、CA hash 获取或认证文件读取命令。
- 现有 `deploy-node` 流程仍使用原有的 master 参数获取逻辑。
- 新节点成功加入后，执行 `kubectl get nodes` 能看到该节点并最终处于 `Ready` 状态。
