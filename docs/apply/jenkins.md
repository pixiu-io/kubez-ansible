# Jenkins 安装

### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- StorageClass。

### 开启 Jenkins 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_jenkins: "no"` 的注释，并设置为 `"yes"`
    ```shell
    ##################
    # Jenkins Options
    ##################
    enable_jenkins: "yes"
    # 配置 jenkins 实例运行的命名空间
    #jenkins_namespace: "{{ kubez_namespace }}"
    # 配置 jenkins 需要使用的 StorageClass 名称，本例中 StorageClass 为 managed-nfs-storage
    jenkins_storage_class: managed-nfs-storage
    # 配置 jenkins 需要使用的存储大小
    jenkins_storage_size: 18Gi
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
    # jenkins pvc 分配成功
    [root@pixiu tmp]# kubectl get pvc -n pixiu-system  jenkins
    NAME      STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          AGE
    jenkins   Bound    pvc-c69ddac3-5b5e-4a2f-82bd-d2405e106d92   18Gi       RWO            managed-nfs-storage   22s

    # jenkins pod 均运行正常
    [root@pixiu tmp]# kubectl get pod -n pixiu-system  jenkins-0
    NAME        READY   STATUS     RESTARTS   AGE
    jenkins-0   1/1     Running    0          65s
    ```

5. 访问 Jenkins
    ```shell
    # 获取 Jenkins 的访问地址
    [root@pixiu tmp]# kubectl get svc -n pixiu-system  jenkins
    NAME      TYPE       CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
    jenkins   ClusterIP  10.254.231.203   <none>       8080/TCP        4h22m
    ```

    > 如果 `type` 的值为 `ClusterIP`，则需要修改 `type` 的值为 `NodePort`，执行 `kubectl edit svc jenkins -n pixiu-system` 修改 `type` 的值为 `NodePort` (原本为`ClusterIP`), 保存退出。执行 `kubectl get svc -n pixiu-system  jenkins` 查看 `NodePort` 的值

    ```shell
    # 查看 NodePort 的值
    [root@pixiu tmp]# kubectl get svc -n pixiu-system  jenkins
    NAME      TYPE       CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
    jenkins   NodePort   10.254.231.203   <none>       8080:30022/TCP   4h27m
    ```

    此时 Jenkins 的访问地址为 `公网ip:30022`，即可访问到 Jenkins. 账号密码为 `admin`/`admin123456`。如果你修改过密码，请使用修改后的密码。·