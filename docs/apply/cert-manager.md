# Cert-manager 安装

### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)

### 开启 Cert-manager 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_cert_manager: "no"` 的注释，并设置为 `"yes"`

3. 执行安装命令（根据实际情况选择）
    ```shell
    # 单节点集群场景
    kubez-ansible apply

    # 高可用集群场景
    kubez-ansible -i multinode apply
    ```

4. 部署完验证
    ```
    # kubectl get pod -n pixiu-system
    NAME                                       READY   STATUS    RESTARTS   AGE
    cert-manager-584f85f6cf-wkbbc              1/1     Running   0          39s
    cert-manager-cainjector-6c58576757-gp4bw   1/1     Running   0          39s
    cert-manager-webhook-75d68f6fb9-8q2sq      1/1     Running   0          39s
    ```
