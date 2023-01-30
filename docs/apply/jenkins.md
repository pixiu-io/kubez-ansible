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
    [root@kirin tmp]# kubectl get pvc -n pixiu-system  jenkins
    NAME      STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          AGE
    jenkins   Bound    pvc-c69ddac3-5b5e-4a2f-82bd-d2405e106d92   18Gi       RWO            managed-nfs-storage   22s
      
    # jenkins pod 均运行正常
   
    [root@kirin tmp]# kubectl get pod -n pixiu-system  jenkins-0
    NAME        READY   STATUS     RESTARTS   AGE
    jenkins-0   1/1     Running    0          65s
    ```
