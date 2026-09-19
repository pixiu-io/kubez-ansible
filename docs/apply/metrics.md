# Metrics Server 安装

### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- 当前清单对齐官方 [metrics-server](https://github.com/kubernetes-sigs/metrics-server) **v0.7.2**（`components.yaml`），部署命名空间为 `kube-system`，镜像走 `app_image_repository`；官方兼容矩阵要求 Kubernetes **1.27+**

### 官方镜像列表

| 用途 | 官方镜像 |
|------|----------|
| Metrics Server | `registry.k8s.io/metrics-server/metrics-server:v0.7.2` |

kubez-ansible 默认经 `app_image_repository` 映射为：

| 用途 | 私有仓库镜像（默认） |
|------|----------------------|
| Metrics Server | `ccr.ccs.tencentyun.com/pixiucloud/metrics-server:v0.7.2` |

### 安装 Metrics Server 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 配置是默认安装，如果不安装，则取消 `enable_metrics_server: "yes"` 的注释，并设置为 `"no"`
    - 注意：安装好再设置成no，再执行第3步安装命令是无效的
    ```shell
    #######################
    # Metrics Server Options
    #######################
    enable_metrics_server: "yes"
    ```

3. 执行安装命令（根据实际情况选择）
    ```shell
    # 单节点集群场景
    kubez-ansible apply

    # 多节点&高可用集群场景
    kubez-ansible -i multinode apply
    ```

4. 部署完验证
    ```shell
    # 所有的 `metrics pod` 均运行正常
    [root@k8s-1 ~]# kubectl get pod -n kube-system
     NAME                              READY   STATUS    RESTARTS   AGE
     metrics-server-xxxxx              1/1     Running   0          22m
    [root@k8s-1 ~]# kubectl top node
     NAME    CPU(cores)   CPU%   MEMORY(bytes)   MEMORY%
     k8s-1   514m         13%    9226Mi          59%
     k8s-2   528m         13%    11480Mi         73%
     k8s-3   448m         11%    10848Mi         69%
    ```
