# MongoDB-Operator 安装

### 依赖条件
- 运行正常的 `kubernetes` ( v1.21+ )环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- 集群已安装 `OLM` 组件。安装手册参考 [OLM安装](../paas/olm.md)
- StorageClass

### 开启 MongoDB-Operator 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_mongodb: "no"` 的注释，并设置为 `"yes"`
    ```shell
    ####################
    # MongoDB Options
    ####################
    enable_mongodb: "yes"

    mongodb_name: mongodb
    mongodb_namespace: operators
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
    # mongodb 已注册至集群中
    [root@VM-16-5-centos manifests]# kubectl get deploy,csv -n operators
    NAME                               READY   UP-TO-DATE   AVAILABLE   AGE
    deployment.apps/mongodb-operator   1/1     1            1           5m16s

    NAME                                                                 DISPLAY            VERSION   REPLACES   PHASE
    clusterserviceversion.operators.coreos.com/mongodb-operator.v0.3.0   MongoDB Operator   0.3.0                Succeeded
    ```

   至此 `MongoDB Operator` 已安装至集群中, 接下来展示 `MongoDB` 实例的创建。

### 创建 MongoDB CR 实例
1. 修改 `yaml` 文件（根据实际情况选择具体参数）
```yaml
   ---
   apiVersion: v1
   kind: Secret
   type: Opaque
   metadata:
     name: mongodb-secret
   data:
     username: YWRtaW4=
     password: MTIzNDU2
   ---
   apiVersion: opstreelabs.in/v1alpha1
   kind: MongoDBCluster
   metadata:
     name: mongodb
   spec:
     clusterSize: 3
     kubernetesConfig:
       image: 'quay.io/opstree/mongo:v5.0.6'
       imagePullPolicy: IfNotPresent
       securityContext:
         fsGroup: 1001
     storage:
       accessModes:
         - ReadWriteOnce
       storageSize: 1Gi
       storageClass: managed-nfs-storage
     mongoDBSecurity:
       mongoDBAdminUser: admin
       secretRef:
         name: mongodb-secret
         key: password
   ```
- 修改 `storageClassName` 为实际存在的 storageClass
- 修改 `storage` 为实际需要的大小

2. 执行 kubectl apply 进行实例安装
   ```shell
   # create-mongodb-cluster.yaml 为步骤1展示的内容
   [root@VM-16-5-centos manifests]# kubectl apply -f create-mongodb-cluster.yml
   secret/mongodb-secret unchanged
   mongodbcluster.opstreelabs.in/mongodb created
   ```

3. 部署完验证
   ```shell
   # pod , pv , pvc 均运行正常
   [root@VM-16-5-centos manifests]# kubectl get po,pv,pvc
   NAME                    READY   STATUS    RESTARTS   AGE
   pod/mongodb-cluster-0   1/1     Running   0          2m17s
   pod/mongodb-cluster-1   1/1     Running   0          45s
   pod/mongodb-cluster-2   1/1     Running   0          28s

   NAME                                                        CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                                       STORAGECLASS          REASON   AGE
   persistentvolume/pvc-7a08ce1d-0f2c-40dd-8481-1891f123ca8f   1Gi        RWO            Delete           Bound    default/mongodb-cluster-mongodb-cluster-1   managed-nfs-storage            45s
   persistentvolume/pvc-8135dac1-edac-41e7-a834-a82f1840548f   1Gi        RWO            Delete           Bound    default/mongodb-cluster-mongodb-cluster-0   managed-nfs-storage            2m17s
   persistentvolume/pvc-a36504e9-88c1-4a6b-8bac-5841fdafe261   1Gi        RWO            Delete           Bound    default/mongodb-cluster-mongodb-cluster-2   managed-nfs-storage            28s

   NAME                                                      STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          AGE
   persistentvolumeclaim/mongodb-cluster-mongodb-cluster-0   Bound    pvc-8135dac1-edac-41e7-a834-a82f1840548f   1Gi        RWO            managed-nfs-storage   2m17s
   persistentvolumeclaim/mongodb-cluster-mongodb-cluster-1   Bound    pvc-7a08ce1d-0f2c-40dd-8481-1891f123ca8f   1Gi        RWO            managed-nfs-storage   45s
   persistentvolumeclaim/mongodb-cluster-mongodb-cluster-2   Bound    pvc-a36504e9-88c1-4a6b-8bac-5841fdafe261   1Gi        RWO            managed-nfs-storage   28s

   # 进入 pod 验证
   [root@VM-16-5-centos manifests]# kubectl exec -it mongodb-cluster-0  -- /bin/bash
   mongo@mongodb-cluster-0:/data/db$ mongo
   MongoDB shell version v5.0.6
   connecting to: mongodb://127.0.0.1:27017/?compressors=disabled&gssapiServiceName=mongodb
   Implicit session: session { "id" : UUID("37dd9d46-b23e-4219-a396-af7a8a823730") }
   MongoDB server version: 5.0.6
   ================
   Warning: the "mongo" shell has been superseded by "mongosh",
   which delivers improved usability and compatibility.The "mongo" shell has been deprecated and will be removed in
   an upcoming release.
   For installation instructions, see
   https://docs.mongodb.com/mongodb-shell/install/
   ================
   Welcome to the MongoDB shell.
   For interactive help, type "help".
   For more comprehensive documentation, see
   https://docs.mongodb.com/
   Questions? Try the MongoDB Developer Community Forums
   https://community.mongodb.com
   > user pixiuDB
   uncaught exception: SyntaxError: unexpected token: identifier :
   @(shell):1:5
   >
   ```

4. 详细文档
   ```shell
   https://github.com/chenghongxi/kubernetes-learning/blob/master/olm/mongodb-operators/README.md
   ```
