# Dify 安装

### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- StorageClass。

### 开启 dify 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_dify: "no"` 的注释，并设置为 `"yes"`
    ```shell
    ##############
    # Dify Options
    ##############
    enable_dify: "yes"
    #dify_namespace: "{{ kubez_namespace }}"

    # Storage class to be used
    #dify_storage_class: managed-nfs-storage

    # helm 仓库配置项
    #dify_repo_name: "{{ default_repo_name }}"
    #dify_repo_url: "{{ default_repo_url }}"
    #dify_path: pixiuio/dify
    #dify_version: 1.4.0
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
    # dify pvc 分配成功
    root@k8s-master-1:~# kubectl get  pvc -n pixiu-system
    NAME                     STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          AGE
    data-dify-postgresql-0   Bound    pvc-92d8a160-1e19-4934-8ca0-1e6884e64b76   8Gi        RWX            managed-nfs-storage   26h
    data-dify-redis-0        Bound    pvc-280337f1-f108-4b58-97b2-c52c423c4027   5Gi        RWX            managed-nfs-storage   26h
    data-dify-weaviate-0     Bound    pvc-4ca2a1a6-9ab1-43c8-843e-03de6063a9a3   10Gi       RWX            managed-nfs-storage   26h
    dify-pvc                 Bound    pvc-f5702ad5-b6f2-48cc-826d-9ca6aa5e58e1   10Gi       RWX            managed-nfs-storage   26h

    # dify pod 均运行正常
    root@k8s-master-1:~# kubectl  get po -n pixiu-system  | grep  dify
    dify-api-0                                     1/1     Running   0             26h
    dify-nginx-cd947cdcd-prkcv                     1/1     Running   0             26h
    dify-plugin-daemon-78f5c76b8f-fb86c            1/1     Running   3 (26h ago)   26h
    dify-postgresql-0                              1/1     Running   0             26h
    dify-redis-0                                   1/1     Running   0             26h
    dify-sandbox-58967f9fb7-v2nfh                  1/1     Running   0             26h
    dify-ssrf-proxy-5d454c94d5-2czpt               1/1     Running   0             26h
    dify-weaviate-0                                1/1     Running   0             26h
    dify-web-754f4c868d-jwzqh                      1/1     Running   0             26h
    dify-worker-0                                  1/1     Running   0             26h
    ```

5. 访问 `dify`
    ```shell
    # 获取 dify 的 service 信息
    [root@pixiu tmp]# kubectl get svc -n pixiu-system  dify
    root@k8s-master-1:~# kubectl  get  svc -n pixiu-system |  grep dify
    dify-api                ClusterIP   10.254.14.242    <none>        5001/TCP            26h
    dify-nginx              ClusterIP   10.254.55.184    <none>        80/TCP              26h
    dify-plugin-daemon      ClusterIP   10.254.74.178    <none>        5002/TCP,5003/TCP   26h
    dify-postgresql         ClusterIP   10.254.133.137   <none>        5432/TCP            26h
    dify-redis              ClusterIP   10.254.96.227    <none>        6379/TCP            26h
    dify-sandbox            ClusterIP   10.254.160.56    <none>        8194/TCP            26h
    dify-ssrf-proxy         ClusterIP   10.254.131.165   <none>        3128/TCP            26h
    dify-weaviate           ClusterIP   10.254.138.85    <none>        8080/TCP            26h
    dify-web                ClusterIP   10.254.244.97    <none>        3000/TCP            26h
    dify-worker             ClusterIP   10.254.247.14    <none>        5001/TCP            26h

    # 如果 dify service 不是 NodePort 类型，则手动调整成 NodePort 类型
    # kubectl edit svc dify-nginx -n pixiu-system
      ...
      sessionAffinity: None
      type: NodePort

    # 查看 NodePort 的值
    root@k8s-master-1:~# kubectl  get  svc -n pixiu-system   dify-nginx
    NAME         TYPE       CLUSTER-IP      EXTERNAL-IP   PORT(S)        AGE
    dify-nginx   NodePort   10.254.55.184   <none>        80:30814/TCP   26h

    # 此时 dify 的访问地址为 `公网ip:30814`，即可访问到 dify.
    ```
