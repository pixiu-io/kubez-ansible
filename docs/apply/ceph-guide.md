# Ceph guide

### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- 运行正常的 `ceph` 集群。 安装手册参考 [ceph](https://docs.ceph.com/en/quincy/install/)

### 创建 `pool` 和 `client auth`
1. 登陆到 `ceph` 集群的 `monitor` 节点，为 `kubernetes` 创建 `pool` 和 `client auth` (现假设pool name为kube)
    ```bash
    ceph osd pool create kube 8 8
    ceph auth add client.kube mon 'allow r' osd 'allow rwx pool=kube'
    ```

2. 获取 `ceph` 集群 `admin` 和新建 pool `kube` 的 `auth key`
    ``` bash
    ceph auth get-key client.admin | base64 （记录回显值为admin_key，后续步骤需要用）
    ceph auth get-key client.kube | base64 （记录回显值为pool_key，后续步骤需要用）
    ```

### 开启 `rbd_provisioner` 组件
1. 登陆到部署节点，编辑 `/etc/kubez/globals.yml`
    ``` bash
    enable_rbd_provisioner: "yes"

    pool_name: kube
    monitors: monitor_ip:port (port默认为6789)
    admin_key: admin_key
    pool_key: pool_key
    ```

2. 执行如下命令完成 `external ceph` 集成.
    ```bash
    # multinode
    kubez-ansible -i multinode apply

    # all-in-one
    kubez-ansible apply
    ```

3. 部署完验证
    ```bash
    kubectl apply -f examples/test-rbd.yaml

    kubectl get pvc
    NAME       STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
    test-rbd   Bound    pvc-487cf629-24e8-4889-a977-dc8ac6c48d22   1Gi        RWO            rbd            25m

    rbd ls kube
    kubernetes-dynamic-pvc-d4a56035-4a94-11ea-aa72-d23b78a708e0
    ```
