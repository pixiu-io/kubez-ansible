# Postgres-Operator 安装

### 依赖条件:
- 运行正常的 `kubernetes` ( v1.17+ )环境,并且集群已安装 `OLM`。如何安装 `OLM`，请移步: [OLM安装](../paas/olm.md)

### 开启 Postgres-Operator 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_postgres-operator: "no"` 的注释，并设置为 `"yes"`
    ```shell
    ####################################
    # Postgres-Operator Options
    ####################################
    enable_postgres-operator: "yes"
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
    # postgres-operator 已注册至集群中
    kubectl get sub -n operators
   
至此 `Postgres-Operator CRD` 已安装至集群中, 接下来可通过外部一些yml文件,来具体安装 `Postgres-Operator` 的具体 `CR` 实例

### 开启 Postgres-Operator CR 实例
1. 执行命令安装（根据实际情况选择具体参数）
   ```shell
   kubectl apply -f https://raw.githubusercontent.com/chenghongxi/kubernetes-learning/master/olm/postgres-Operators/yml/create-postgres-cluster.yaml
   ```

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
