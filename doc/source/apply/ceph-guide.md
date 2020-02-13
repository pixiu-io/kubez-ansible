# Ceph guide

1. 登陆到ceph集群的monitor节点，为kubernetes创建pool和client auth(现假设pool name为kube)

    ``` bash
    ceph osd pool create kube 8 8
    ceph auth add client.kube mon 'allow r' osd 'allow rwx pool=kube'
    ```

2. 获取ceph集群 `admin` 和新建 pool `kube` 的auth key

    ``` bash
    ceph auth get-key client.admin | base64 （记录回显值为admin_key，后续步骤需要用）
    ceph auth get-key client.kube | base64 （记录回显值为pool_key，后续步骤需要用）
    ```

3. 登陆到部署节点，编辑 `/etc/kubernetes-ansible/globals.yml`

    ``` bash
    enable_rbd_provisioner: "yes"

    pool_name: kube
    monitors: monitor_ip:port (port默认为6789)
    admin_key: admin_key
    pool_key: pool_key
    ```

4. 执行如下命令完成 `external ceph` 集成.

    ``` bash
    # multinode
    kubernetes-ansible -i multinode apply

    # all-in-one
    kubernetes-ansible apply
    ```

5. 验证，得到类似回显

    ``` bash
    kubectl apply -f test-rbd.yaml

    kubectl get pvc
    NAME       STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
    test-rbd   Bound    pvc-487cf629-24e8-4889-a977-dc8ac6c48d22   1Gi        RWO            rbd            25m

    rbd ls kube
    kubernetes-dynamic-pvc-d4a56035-4a94-11ea-aa72-d23b78a708e0
    ```
