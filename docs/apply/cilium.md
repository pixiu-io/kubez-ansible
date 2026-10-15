# Cilium & Hubble

Cilium 与 flannel / calico 一样，通过 `cni` 选择，在集群 `deploy`（或后续 `apply`）时以 **kubectl apply** 安装。三者互斥。

默认版本：**v1.20.2**（写死在模板）。镜像仓库复用全局 `app_image_repository`。

对齐官方默认：

- 保留 kube-proxy（未启用 kube-proxy replacement）
- Hubble（agent 内）默认开启
- **Hubble Relay / UI 默认不安装**，可在部署完成后按需启用

## 依赖条件

- Kubernetes >= 1.21（Cilium 1.20 Chart / 官方要求）
- Linux kernel >= 5.10（Cilium 1.20 官方建议）
- 集群安装手册参考 [多节点](../install/multinode.md) 或 [单节点](../install/all-in-one.md)

## 开启 Cilium

1. 编辑 `/etc/kubez/globals.yml`

2. 设置容器网络插件为 cilium（镜像仓库沿用 `app_image_repository`）：

   ```yaml
   # Kubernetes network cni options
   cni: "cilium"

   app_image_repository: "ccr.ccs.tencentyun.com/pixiucloud"
   ```

3. 执行安装命令（根据实际情况选择）

   ```shell
   # 新建集群（推荐）
   kubez-ansible deploy
   # 或多节点
   kubez-ansible -i multinode deploy

   # 已有集群且尚未安装其他 CNI 时
   kubez-ansible apply
   # 或多节点
   kubez-ansible -i multinode apply
   ```

   > 若集群已运行 flannel 或 calico，切换到 cilium 需要自行清理旧 CNI，不建议直接改 `cni` 后 apply。

4. 部署完验证

   ```shell
   kubectl get pods -n kube-system | grep cilium
   ```

## 部署后启用 Hubble Relay / UI（可选）

```shell
cilium hubble enable          # 启用 Relay（默认）
cilium hubble enable --ui     # 同时启用 UI
```

## 镜像说明

需提前将下列镜像同步到 `app_image_repository`：

- `cilium:v1.20.2`
- `operator-generic:v1.20.2`
- `cilium-envoy`（manifest 中固定 tag）

启用 Relay / UI 后还需 `hubble-relay`、`hubble-ui`、`hubble-ui-backend`。
