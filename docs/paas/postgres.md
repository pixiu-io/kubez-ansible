# Postgres-Operator 安装

### 依赖条件:
- 运行正常的 `kubernetes` ( v1.17+ )环境，装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- 集群已安装 `OLM`。如何安装 `OLM`，请移步: [OLM安装](../paas/olm.md)
- StorageClass

### 开启 Postgres-Operator 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_postgres: "no"` 的注释，并设置为 `"yes"`
    ```shell
    ####################
    # Postgres Options #
    ####################
    enable_postgres: "yes"
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
    # postgres 已注册至集群中
    kubectl get csv -n operators
    [root@VM-4-3-centos ~]# kubectl get deploy -n operators
    NAME    READY UP-TO-DATE AVAILABLE AGE
    pgo     1/1   1          1      87m 
   
至此 `Postgres CRD` 已安装至集群中, 接下来可通过外部一些yml文件,来具体安装 `Postgres` 的具体 `CR` 实例

### 开启 Postgres-Operator CR 实例
1. 执行命令安装（根据实际情况选择具体参数）
   ```shell
   kubectl apply -f https://raw.githubusercontent.com/chenghongxi/kubernetes-learning/master/olm/postgres-Operators/yml/create-postgres-cluster.yaml
   storageclass.storage.k8s .io/redis-storage created
   storageclass.storage.k8s .io/backups-redis-storage created
   persistentvolume/pv created
   persistentvolume/pv1 created
   postgrescluster.postgres-operator.crunchydata.com/hippo created
   ```
   
   注: 如遇网络问题无法 `apply` , 可通过下方 `yaml` 文件创建

   [postgres-cluster.yaml](https://raw.githubusercontent.com/chenghongxi/kubernetes-learning/master/olm/postgres-Operators/yml/create-postgres-cluster.yaml)

2. 部署完验证
   ```shell
   # pod 均运行正常
   [root@VM-4-3-centos ~]# kubectl get sts 
   NAME                                    READY  AGE
   statefulset.apps/hippo-instance1-zw2j   1/1    119s
   statefulset.apps/hippo-repo-host119s    1/1    119s
   ```
   
3. 详细文档
   ```shell
   https://github.com/chenghongxi/kubernetes-learning/blob/master/olm/postgres-Operators/README.md
   ```
