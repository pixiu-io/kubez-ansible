# NFS provisioner 安装

### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)

### 开启 NFS 组件
1. 默认 `nfs` 是开启状态

2. 关闭 `nfs provisioner`
    ```shell
    # 取消 `enable_nfs: "yes" 的注释, 并设置为 "no" 可关闭 nfs
    #######################
    # StorageClass Options
    #######################
    enable_nfs: "no"
    ```

3. `nfs` 会随着 `kubernetes` 集群一同安装，使用 `nfs` 前需在 `kube-apiserver `的启动命令中添加 `--feature-gates=RemoveSelfLink=false`

4. 等待 `kube-apiserver` 启动，完成配置修改
   ```shell
   # 命令行正常回显
   kubectl get node
   ```
