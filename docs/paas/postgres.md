# Postgres-Operator 安装

### 依赖条件
- 运行正常的 `kubernetes` ( v1.21+ )环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- 集群已安装 `OLM` 组件。安装手册参考 [OLM安装](../paas/olm.md)
- StorageClass

### 开启 Postgres-Operator 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_postgres: "no"` 的注释，并设置为 `"yes"`
    ```shell
    ##################
    # Postgres Options
    ##################
    enable_postgres: "yes"

    postgres_name: postgres
    postgress_namespace: operators
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
    [root@VM-16-5-centos ~]# kubectl get deploy,csv -n operators
    NAME                  READY   UP-TO-DATE   AVAILABLE   AGE
    deployment.apps/pgo   1/1     1            1           116m

    NAME                                                                 DISPLAY                           VERSION   REPLACES                  PHASE
    clusterserviceversion.operators.coreos.com/postgresoperator.v5.3.0   Crunchy Postgres for Kubernetes   5.3.0     postgresoperator.v5.2.0   Succeeded
   ```

至此 `Postgres Operator` 已安装至集群中, 接下来展示 `Postgres` 实例的创建。

### 创建 Postgres CR 实例
1. 修改 `yaml` 文件（根据实际情况选择具体参数）
   ```yaml
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
           storageClassName: "managed-nfs-storage"
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
                 storageClassName: "managed-nfs-storage"
                 resources:
                   requests:
                     storage: 1Gi
   ```
- 修改 `storageClassName` 为实际存在的 storageClass
- 修改 `storage` 为实际需要的大小

2. 执行 kubectl apply 进行实例安装
   ```shell
   # create-postgres-cluster.yaml 为步骤1展示的内容
   [root@VM-16-5-centos manifests]# kubectl apply -f create-postgres-cluster.yaml
   postgrescluster.postgres-operator.crunchydata.com/hippo created
   ```

3. 部署完验证
   ```shell
   # pod 均运行正常
   [root@VM-16-5-centos manifests]# kubectl get po,pv,pvc
   NAME                          READY   STATUS      RESTARTS   AGE
   pod/hippo-backup-7gcg-zx684   0/1     Completed   0          3m26s
   pod/hippo-instance1-crq2-0    4/4     Running     0          7m37s
   pod/hippo-repo-host-0         2/2     Running     0          7m37s

   NAME                                                        CAPACITY    ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                                 STORAGECLASS          REASON   AGE
   persistentvolume/pvc-ba6958d7-3fc7-4a9c-bd69-4cb9d8d7c291   1Gi        RWO            Delete           Bound    default/hippo-repo1                   managed-nfs-storage            7m37s
   persistentvolume/pvc-d2bcb0b3-9d7d-4e11-8bae-a74e62b3ad64   1Gi        RWO            Delete           Bound    default/hippo-instance1-crq2-pgdata   managed-nfs-storage            7m37s

   NAME                                                STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          AGE
   persistentvolumeclaim/hippo-instance1-crq2-pgdata   Bound    pvc-d2bcb0b3-9d7d-4e11-8bae-a74e62b3ad64   1Gi        RWO            managed-nfs-storage   7m37s
   persistentvolumeclaim/hippo-repo1                   Bound    pvc-ba6958d7-3fc7-4a9c-bd69-4cb9d8d7c291   1Gi        RWO            managed-nfs-storage   7m37s

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
