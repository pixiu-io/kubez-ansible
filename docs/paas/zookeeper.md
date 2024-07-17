# Zookeeper 安装

### 依赖条件
- 运行正常的 `kubernetes` ( v1.21+ )环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- StorageClass

### 开启 Zookeeper 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_zookeeper: "no"` 的注释，设置为 `"yes"`，并取消如下参数注释
    ```shell
   ######################
   # zookeeper Options
   ######################
   # https://artifacthub.io/packages/helm/bitnami/zookeeper
   enable_zookeeper: "yes"
   zookeeper_namespace: pixiu-system
   # 集群数量需要是单数
   zookeeper_replica: 1
   # 请求资源
   zookeeper_requests_cpu: 2
   zookeeper_requests_memory: 4Gi
   # 持久化存储配置
   zookeeper_storage_size: 8Gi
   zookeeper_storage_class: managed-nfs-storage
   # 版本配置
   zookeeper_version: 11.4.9
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
    root@VM-48-16-ubuntu:~# kubectl get pod -n pixiu-system zookeeper-0
   NAME          READY   STATUS    RESTARTS   AGE
   zookeeper-0   1/1     Running   0          58s

注意 如pod启动失败Error：
   ```
      #存储目录授权(因为由于 Bitnami zookeeper 容器是非根容器，因此 id 为 1001 的用户需要在您挂载的本地文件夹中拥有写权限)
      chown -R 1001:1001 {实际目录}
   ```
至此 `zookeeper` 已安装至集群中。
