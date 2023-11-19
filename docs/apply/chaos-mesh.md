# Chaos Mesh 安装

### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- StorageClass。

### 开启 Chaos Mesh 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_chaos_mesh: "no"` 的注释，并设置为 `"yes"`
    ```shell
    ####################
    # Chaos Mesh Options
    ####################
    enable_chaos_mesh: "yes"
    #chaos_mesh_name: chaos-mesh
    #chaos_mesh_namespace: "{{ kubez_namespace }}"
    ```

3. 执行安装命令（根据实际情况选择）
    ```shell
    # 单节点集群场景
    kubez-ansible apply

    # 高可用集群场景
    kubez-ansible -i multinode apply
    ```

4. 部署完验证
    ```shell
    # kubectl get pod -n pixiu-system
    chaos-controller-manager-6f75cdf4f9-62fmh   0/1     ContainerCreating   0             6m24s
    chaos-controller-manager-6f75cdf4f9-klxq2   0/1     ContainerCreating   0             6m24s
    chaos-controller-manager-6f75cdf4f9-mpvfd   0/1     ContainerCreating   0             6m24s
    chaos-daemon-mpqkx                          0/1     ContainerCreating   0             6m24s
    chaos-daemon-znpxg                          0/1     ContainerCreating   0             6m24s
    chaos-dashboard-7684ff75f4-67lrd            0/1     ContainerCreating   0             6m24s
    chaos-dns-server-76d5d8f776-pbpj7           0/1     ContainerCreating   0             6m24s
    ```
