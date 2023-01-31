# Postgres-Operator 安装

### 依赖条件:
- 运行正常的 `kubernetes` ( v1.21+ )环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- 集群已安装 `OLM` 组件。安装手册参考 [OLM安装](../paas/olm.md)
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
   
至此 `Postgres Operator` 已安装至集群中, 接下来展示 `Postgres` 实例的创建。

### 创建 Postgres CR 实例
1. 修改 `yaml` 文件（根据实际情况选择具体参数）
   ```yaml
   ---
   apiVersion: v1
   kind: PersistentVolume
   metadata:
     name: pv
   spec:
     capacity:
       storage: 8Gi
     volumeMode: Filesystem
     accessModes:
       - ReadWriteOnce
     persistentVolumeReclaimPolicy: Delete
     storageClassName: {{ storageClassName }}
     local:
       path: /postgres-operators

   ---
   apiVersion: v1
   kind: PersistentVolume
   metadata:
     name: pv1
   spec:
     capacity:
       storage: 8Gi
     volumeMode: Filesystem
     accessModes:
       - ReadWriteOnce
     persistentVolumeReclaimPolicy: Delete
     storageClassName: {{ storageClassName }}
     local:
       path: /postgres-operators

   ---
   apiVersion: postgres-operator.crunchydata.com/v1beta1
   kind: PostgresCluster
   metadata:
     name: hippo
   spec:
     image: registry.developers.crunchydata.com/crunchydata/crunchy-postgres:ubi8-14.5-1
     postgresVersion: 14
     instances:
       - name: instance1
         replicas: 1
         dataVolumeClaimSpec:
           accessModes:
             - "ReadWriteOnce"
           storageClassName: "postgres-storage"
           resources:
             requests:
               storage: 1Gi
     backups:
       pgbackrest:
         image: registry.developers.crunchydata.com/crunchydata/crunchy-pgbackrest:ubi8-2.40-1
         repos:
           - name: repo1
             volume:
               volumeClaimSpec:
                 accessModes:
                   - "ReadWriteOnce"
                 storageClassName: "backups-postgres-storage"
                 resources:
                   requests:
                     storage: 1Gi
   ```
- 修改 `storageClassName` 为实际存在的 storageClass
- 修改 `storage` 为实际需要的大小

2. 执行 kubectl apply 进行实例安装  
   ```shell
   # create-postgres-cluster.yaml 为步骤1展示的内容
   kubectl apply -f create-postgres-cluster.yaml
   storageclass.storage.k8s .io/redis-storage created
   storageclass.storage.k8s .io/backups-redis-storage created
   persistentvolume/pv created
   persistentvolume/pv1 created
   postgrescluster.postgres-operator.crunchydata.com/hippo created
   ```   

3. 部署完验证
   ```shell
   # pod 均运行正常
   [root@VM-4-3-centos ~]# kubectl get sts 
   NAME                                    READY  AGE
   statefulset.apps/hippo-instance1-zw2j   1/1    119s
   statefulset.apps/hippo-repo-host119s    1/1    119s
   
   # 进入 pod 验证
   [root@VM-4-3-centos ~]# kubectl exec -it hippo-instance1-zw2j-0 -- /bin/bash
   Defaulted container "database"out of: database, replication-cert-copy, pgbackrest, pgbackrest-config, postgres-startup (init), nss-wrapper-init (init)
   psql (14.5)
   Type "help" for help.
   postgres=#
   ```
4. 详细文档
   ```shell
   https://github.com/chenghongxi/kubernetes-learning/blob/master/olm/postgres-Operators/README.md
   ```
