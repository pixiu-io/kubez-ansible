# 高可用部署 -- multinode

1. 部署前准备 -- [前提条件](prerequisites.md)

2. 配置工作目录下的 [multinode](https://github.com/caoyingjunz/kubez-ansible/blob/master/ansible/inventory/multinode) , 根据实际情况添加主机信息, 并完成如下配置

    ``` bash
    a. 打通部署节点到其他节点的免密登陆

    b. 配置部署节点的 /etc/hosts , 添加 kubernetes 节点的ip和主机名解析

    c. multinode 配置格式，推荐：
       # 如果 cri 选择 docker，则仅需配置 [docker-master] 和 [docker-node]
       [docker-master]
       kube01

       [docker-node]
       kube02

       # 如果 cni 选择 containerd，则仅需配置 [containerd-master] 和 [containerd-node]
       [containerd-master]
       kube01

       [containerd-node]
       kube02

       [storage]
       kube01
    ```

3. 执行如下命令，进行kubernetes的依赖安装

    ``` bash
    kubez-ansible -i multinode bootstrap-servers
    ```

4. （可选）使用离线安装模式 --[开启本地私有仓库](setup-registry.md)

5. 根据实际需要，调整配置文件 `/etc/kubez/globals.yml`

    ```bash
    enable_kubernetes_ha: "yes"  # (可选)启用多控高可用, 需保证 multinode 的 control 组为奇数

    cluster_cidr: "172.30.0.0/16"  # pod network
    service_cidr: "10.254.0.0/16"  # service network

    # network cni, 现支持flannel 和 calico, 默认是 flannel
    enable_calico: "no"

    enable_registry: "yes"  # （可选)开启私有仓库
    registry_server: `registry_server_ip:4000`
    ```

6. 执行如下命令，进行 `kubernetes` 的集群安装

    ``` bash
    kubez-ansible -i multinode deploy
    ```

7. 生成 `kubernetes` RC 文件 `.kube/config`
   ``` bash
   kubez-ansible -i multinode post-deploy
   ```

8. 验证环境
   ```bash
   [root@kube01 ~]# kubectl get node
   NAME     STATUS   ROLES                  AGE     VERSION
   kube01   Ready    control-plane,master   21h     v1.23.6
   kube02   Ready    <none>                 21h     v1.23.6
   kube03   Ready    <none>                 3h48m   v1.23.6
   ```
