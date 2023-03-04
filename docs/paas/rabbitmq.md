# Rabbitmq-Operators 安装

### 依赖条件
- 运行正常的 `kubernetes` ( v1.21+ )环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- 集群已安装 `OLM` 组件。安装手册参考 [OLM安装](../paas/olm.md)
- StorageClass
### 开启 Rabbitmq-Operators 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_postgres: "no"` 的注释，并设置为 `"yes"`
    ```shell
    ##################
    # RabbitMQ Options
    ##################
    enable_rabbitmq: "yes"

    rabbitmq_name: rabbitmq
    rabbitmq_namespace: operators
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
    NAME                                 DISPLAY                     version    REPLACES                           PHASE
    rabbitmq-cluster-operator.v2.0.0     RabbitMQ-cluster-operator   2.0.0      rabbitmq-cluster-operator.v1.14.0  Succeeded

至此 `RabbitMQ Operator` 已安装至集群中, 接下来展示 `RabbitMQ` 实例的创建。

### 创建 RabbitMQ CR 实例
1. 修改 `yaml` 文件（根据实际情况选择具体参数）
   ```yaml
   apiVersion: rabbitmq.com/v1beta1
   kind: RabbitmqCluster
   metadata:
     name: rabbitmqcluster-sample
   spec:
     persistence:
       storageClassName: {{ storageClass }}
       storage: 1Gi
   ```
2. 执行 kubectl apply 进行实例安装
   ```shell
   #rabbitmq-cluster-operator.yaml 为步骤1展示的内容
   kubectl apply -f  rabbitmq-cluster-operator.yaml
   ```
3. 部署完验证
   ```shell
   kubectl get po,sc,pv,pvc,secret
   ```
4. 删除资源
- 删除步骤3中的资源
 ```shell
  kubectl delete -f create-rabbitmq-cluster.yaml
  ```
- 删除此Operator
```shell
1. kubectl delete subscription <subscription-name> -n operators
2. kubectl delete clusterserviceversion -n operators
```
5. 详细文档
```shell
https://github.com/chenghongxi/kubernetes-learning/blob/master/olm/rabbitmq-operators/README.md
```
