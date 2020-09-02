# 生产环境部署 -- multinode

1. 部署前准备--[前提条件](prerequisites.md)

2. 配置工作目录下的 `multinode` , 根据实际情况添加主机信息, 并完成如下配置

    ``` bash
    a. 打通部署节点到其他节点的免密登陆

    b. 配置部署节点的 /etc/hosts , 添加kubernetes节点的ip和主机名解析

    c. multinode 配置格式，推荐：
        [control]
        kube0[1:3]

        [compute]
        kube0[4:5]
        kube07
    ```

3. 执行如下命令，进行kubernetes的依赖安装

    ``` bash
    kubernetes-ansible -i multinode  bootstrap-servers
    ```

4. （可选）使用离线安装模式 --[开启本地私有仓库](setup-registry.md)

5. 根据实际需要，调整配置文件 `/etc/kubernetes-ansible/globals.yml`

    ```bash
    enable_kubernetes_ha: "yes"  # (可选)启用多控高可用, 需保证multunode的control组为奇数

    cluster_cidr: "172.30.0.0/16"  # pod network
    service_cidr: "10.254.0.0/16"  # service network

    # network cni, 现支持flannel, calico, 和ovn, 默认是flannel
    enable_calico: "yes"

    enable_registry: "yes"  # （可选)开启私有仓库
    registry_server: `registry_server_ip:4000`
    ```

6. 执行如下命令，进行kubernetes的集群安装

    ``` bash
    kubernetes-ansible -i multinode deploy
    ```
