# Harbor 安装

### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- StorageClass。

### 开启 Harbor 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_harbor: "no"` 的注释，并设置为 `"yes"`
    ```shell
    #################
    # Harbor Options
    #################
    enable_harbor: "yes"
    #harbor_name: harbor
    #harbor_namespace: "{{ kubez_namespace }}"
    # 配置 harbor 需要使用的存储大小
    harbor_storage_size: "18Gi"
    # 配置 admin 用户的密码
    harbor_admin_password: "Harbor12345"
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
    # harbor pvc 分配成功
    [root@pixiu kubez]# kubectl get pvc -n pixiu-system
    NAME                              STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          AGE
    data-harbor-redis-0               Bound    pvc-f5237b46-5748-4ae3-ad8c-90bab1bb0534   15Gi       RWO            managed-nfs-storage   16d
    data-harbor-trivy-0               Bound    pvc-dd2e31d5-e36e-426c-b9c4-732fb24295bd   15Gi       RWO            managed-nfs-storage   16d
    data-mariadb-0                    Bound    pvc-f71527d1-bcd1-4a20-b6f4-6a6a3096bcf9   10Gi       RWO            managed-nfs-storage   17d
    database-data-harbor-database-0   Bound    pvc-f278bad4-e51c-429b-b845-b240afc220a2   15Gi       RWO            managed-nfs-storage   16d
    harbor-chartmuseum                Bound    pvc-6e4aa2d1-34ac-40e5-8b52-c5085271b503   19Gi       RWO            managed-nfs-storage   10d
    harbor-jobservice                 Bound    pvc-c9d30ec7-8bbb-470e-bdc0-018dceffa2cc   1Gi        RWO            managed-nfs-storage   10d
    harbor-jobservice-scandata        Bound    pvc-7e1b0a36-ff01-4fce-a43d-c6ce8de64eeb   1Gi        RWO            managed-nfs-storage   10d
    harbor-registry                   Bound    pvc-7d143c4c-c4ce-4846-bfc3-43b6f1333a51   19Gi       RWO            managed-nfs-storage   10d

    # harbor pod 均运行正常
    [root@pixiu kubez]# kubectl get pod -n pixiu-system
    NAME                                             READY   STATUS             RESTARTS           AGE
    harbor-chartmuseum-76ff89f9d4-nlljp              1/1     Running            0                  10d
    harbor-core-55787dc698-qfqhg                     1/1     Running            0                  10d
    harbor-database-0                                1/1     Running            0                  10d
    harbor-jobservice-c797dd585-rgbrs                1/1     Running            3 (10d ago)        10d
    harbor-nginx-fc68874d5-69zw2                     1/1     Running            0                  10d
    harbor-notary-server-5c97479877-8hm9z            1/1     Running            0                  10d
    harbor-notary-signer-7d56fd69c7-drc68            1/1     Running            0                  10d
    harbor-portal-b7d5d9558-988zr                    1/1     Running            0                  10d
    harbor-redis-0                                   1/1     Running            0                  10d
    harbor-registry-dc457dd49-9fpfh                  2/2     Running            0                  10d
    harbor-trivy-0                                   1/1     Running            0                  10d
    ```

5. 功能验证
   TODO
