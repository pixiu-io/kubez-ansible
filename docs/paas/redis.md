# Redis-Operator 安装

### 依赖条件
- 运行正常的 `kubernetes` ( v1.21+ )环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- 集群已安装 `OLM` 组件。安装手册参考 [OLM安装](../paas/olm.md)
- StorageClass

### 开启 Redis-Operator 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_redis: "no"` 的注释，设置为 `"yes"`，并取消如下参数注释
    ```shell
    ###############
    # Redis Options
    ###############
    enable_redis: "yes"

    redis_name: redis
    redis_namespace: operators
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
    # redis 已注册至集群中
    [root@VM-16-5-centos ~]# kubectl get csv -n operators
    NAME                       DISPLAY         VERSION        REPLACES                  PHASE
    redis-operator.v0.12.0     Redis Operator  0.12.0         redis-operator.v0.11.0    Succeeded

至此 `Redis Operator` 已安装至集群中, 接下来展示 `redis` 实例的创建。

### 创建 Redis CR 实例
1. 修改 `yaml` 文件（根据实际情况选择具体参数）
```yaml
apiVersion: redis.redis.opstreelabs.in/v1beta1
kind: RedisCluster
metadata:
  name: redis-cluster
spec:
  clusterSize: 3
  clusterVersion: v7
  securityContext:
    runAsUser: 1000
    fsGroup: 1000
  persistenceEnabled: true
  kubernetesConfig:
    image: 'quay.io/opstree/redis:v7.0.5'
    imagePullPolicy: IfNotPresent
  redisExporter:
    enabled: true
    image: 'quay.io/opstree/redis-exporter:v1.44.0'
    imagePullPolicy: IfNotPresent
  storage:
    # volumeClaimTemplate 自动创建 pvc
    volumeClaimTemplate:
      spec:
        accessModes:
          - ReadWriteOnce
        # storageClassName 自动创建 pv
        storageClassName: managed-nfs-storage
        resources:
          requests:
            storage: 1Gi
```

- 修改 `storageClassName` 为实际存在的 storageClass
- 修改 `storage` 为实际需要的大小

2. 执行 kubectl apply 进行实例安装
   ```shell
   # create-redis-cluster.yaml 为步骤1展示的内容
   [root@VM-16-5-centos manifests]# kubectl apply -f create-redis-cluster.yaml
   rediscluster.redis.redis.opstreebals.in/redis-cluster created
   ```

3. 部署完验证
   ```shell
   # pod 均运行正常
   [root@VM-16-5-centos manifests]# kubectl get po
   NAME                         READY    STATUS        RESTART        AGE
   redis-cluster-follower-0     2/2      Running       0              8m31s
   redis-cluster-follower-1     2/2      Running       0              8m31s
   redis-cluster-follower-2     2/2      Running       0              8m31s
   redis-cluster-leader-0       2/2      Running       0              8m31s
   redis-cluster-leader-1       2/2      Running       0              8m31s
   redis-cluster-leader-2       2/2      Running       0              8m31s
   # 进入 pod 验证
   [root@VM-4-3-centos ~]# kubectl exec -it redis-cluster-leader-0 -- redis-cli -c cluster nodes
   Defaulted container "redis-cluster-leader" out of: redis-cluster-leader, redis-exporter
   2900d323ae4ff71ff4ceab0257196df4167ab6 172,30.142,32:637916379 slave ced685ff9fC309f5e01af69a31fd87a278b59 0 166979307000 3 connected
   ba6857a114e5674eba9d425hdcac56238 172,30.14230:637916379 slave 101d8533d900be6f11af864965729836ea6fhd9 0 1669799306822 1 connected
   101d8533d900be6+11a+8649655779836e36bd9 172.3.142.31:6379@16379 myself,master -9 1669799305000 1 connected 0-5460
   943094C18642604606773c476+c33e840168169 177.39.147.49:637901637g masten -9 16697993979997 connected5161-19922
   ced685++0fc309f5e01af60ca31+d87a2078650 172.39.142.35:6379@16379 master 1669799397099connected 10923-16383
   a2dd5798b448115ab62dab5581570076f41339cC 172.30.142.48:6379@16379 slave 5943094c186426046db773c476fc33e840168169 0 1669799306020 2 connected
   ```
4. 详细文档
   ```shell
   https://github.com/chenghongxi/kubernetes-learning/blob/master/olm/redis-operators/README.md
   ```
