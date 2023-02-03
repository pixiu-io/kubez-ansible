# kafka-Operators 安装

### 依赖条件:
- 运行正常的 `kubernetes` ( v1.21+ )环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- 集群已安装 `OLM` 组件。安装手册参考 [OLM安装](../paas/olm.md)
- StorageClass
### 开启 Rabbitmq-Operators 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_kafka: "no"` 的注释，并设置为 `"yes"`
    ```shell
    ####################
    # kafka Options #
    ####################
    enable_kafka: "yes"
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
    # rabbitmq 已注册至集群中
    kubectl get csv -n operators
    [root@VM-4-3-centos ~]# kubectl get csv -n operators
    NAME                               DISPLAY   VERSION   REPLACES                           PHASE
strimzi-cluster-operator.v0.33.0   Strimzi   0.33.0    strimzi-cluster-operator.v0.32.0   Succeeded

至此 `kafka Operator` 已安装至集群中, 接下来展示 `kafka` 实例的创建。

### 创建 kafka CR 实例
1. 修改 `yaml` 文件（根据实际情况选择具体参数）
   ```yaml
    apiVersion: kafka.strimzi.io/v1beta2
    kind: Kafka
    metadata:
      name: my-cluster
    spec:
      kafka:
        version: 3.3.2
        replicas: 3
        listeners:
          - name: plain
            port: 9092
            type: internal
            tls: false
          - name: tls
            port: 9093
            type: internal
            tls: true
        config:
          offsets.topic.replication.factor: 3
          transaction.state.log.replication.factor: 3
          transaction.state.log.min.isr: 2
          default.replication.factor: 3
          min.insync.replicas: 2
          inter.broker.protocol.version: '3.3'
        storage:
          type: ephemeral
      zookeeper:
        replicas: 3
        storage:
          type: ephemeral
      entityOperator:
        topicOperator: {}
        userOperator: {}

   ```
2. 执行 kubectl apply 进行实例安装  
   ```shell
   #kafka-operator.yaml 为步骤1展示的内容
   kubectl apply -f  kafka-operator.yaml
   ```
3. 部署完验证
   ```shell
   kubectl get po,sc,pv,pvc,secret
   ```
4. 删除资源
- 删除步骤3中的资源
 ```shell
  kubectl delete -f kafka-operator.yaml.yaml
  ```
- 删除此Operator
```shell
1. kubectl delete subscription <subscription-name> -n operators
2. kubectl delete clusterserviceversion -n operators
```
5. 详细文档
```shell
https://github.com/chenghongxi/kubernetes-learning/blob/master/olm/kafka-operators/README.md
```
