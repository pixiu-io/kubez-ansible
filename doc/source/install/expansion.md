# Worker节点扩容

1. 配置工作目录下的 `multinode` , 根据实际情况添加待扩容节点到 `compute` 组, 并完成如下配置

    ``` bash
    a. 打通部署节点到新增节点的免密登陆

    b. 配置部署节点的 /etc/hosts , 添加kubernetes节点的ip和主机名解析

    c. multinode 配置格式，推荐：
        [control]
        kube0[1:3]

        [compute]
        kube0[4:5]
        kube07
        kube08
    ```

2. 执行如下命令，进行kubernetes的依赖安装

    ``` bash
    kubez-ansible -i multinode  bootstrap-servers
    ```

3. 执行如下命令，进行kubernetes的集群安装

    ``` bash
    kubez-ansible -i multinode deploy
    ```
