# Artifactory 安装

### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- StorageClass。

### 开启 Artifactory 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_artifactory: "no"` 的注释，并设置为 `"yes"`
    ```shell
    ######################
    # Artifactory Options
    ######################
    enable_artifactory: "yes"

    # 配置 Artifactory 实例运行的命名空间
    #artifactory_namespace: "{{ kubez_namespace }}"

    # 配置 Artifactory 需要使用的 StorageClass 名称，本例中 StorageClass 为 managed-nfs-storage
    #artifactory_storage_class: managed-nfs-storage

    # 配置 Artifactory 需要使用的存储大小
    #artifactory_size: "20Gi"
    # 配置 PostgreSQL 需要使用的存储大小
    #postgresql_size: "20Gi"
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
    # Artifactory pvc 分配成功
    root@VM-0-9-ubuntu:~# kubectl  get pvc -A
    NAMESPACE      NAME                               STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          AGE
    pixiu-system   artifactory-volume-artifactory-0   Bound    pvc-8e5c28cb-50ee-4306-adb1-4980bd1bc401   10Gi       RWO            managed-nfs-storage   75s
    pixiu-system   data-artifactory-postgresql-0      Bound    pvc-9ade00ca-4cc7-4ef4-be2c-4058d353aa69   10Gi       RWO            managed-nfs-storage   75s

    # 存储目录授权（因为由于 Bitnami PostgreSQL 容器是非根容器，因此 id 为 1001 的用户需要在您挂载的本地文件夹中拥有写权限)
    chown -R 1001:1001  /data/share/pvc-9ade00ca-4cc7-4ef4-be2c-4058d353aa69

    # 您可以在 GitHub 存储库中找到有关这方面的更多信息
    # https://github.com/bitnami/bitnami-docker-postgresql#persisting-your-database

    # 查看 Pod 运行状态
    root@VM-0-9-ubuntu:~# kubectl  get pod -A
    pixiu-system   artifactory-0                                    8/8     Running                 9  (9d ago)     9d
    pixiu-system   artifactory-artifactory-nginx-85ff4d76b5-wwmj6   1/1     Running                 7  (9d ago)     9d
    pixiu-system   artifactory-postgresql-0                         1/1     Running                 15 (9d ago)     9d
    ```

5. 访问 `Artifactory`
    ```shell
    # 获取 Artifactory 的 service 信息
    root@VM-0-9-ubuntu:~# kubectl get svc -n pixiu-system  artifactory
    NAME          TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)             AGE
    artifactory   ClusterIP   10.254.42.238   <none>        8082/TCP,8081/TCP   9d

    # 如果 artifactory service 不是 NodePort 类型，则手动调整成 NodePort 类型
    # root@VM-0-9-ubuntu:~# kubectl edit svc artifactory -n pixiu-system
      ...
      sessionAffinity: None
      type: NodePort

    # 查看 NodePort 的值
    root@VM-0-9-ubuntu:~# kubectl get svc -n pixiu-system  artifactory
    NAME          TYPE       CLUSTER-IP      EXTERNAL-IP   PORT(S)                         AGE
    artifactory   NodePort   10.254.42.238   <none>        8082:32558/TCP,8081:31773/TCP   9d

    # 此时 Artifactory 的访问地址为 公网ip:32558，即可访问到 Artifactory. 账号密码为 "admin/password"。
    ```
