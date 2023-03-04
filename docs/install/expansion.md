# Worker节点扩容

1. 配置工作目录下的 `multinode` , 根据实际情况添加待扩容节点到 `cri` 组, 并完成如下配置
    ``` text
    a. 打通部署节点到新增节点的免密登陆

    b. 配置部署节点的 /etc/hosts , 添加 kubernetes 节点的ip和主机名解析

    c. multinode 配置格式，推荐
       # 如果 cri 选择 docker，则仅需配置 [docker-master] 和 [docker-node]
       [docker-master]
       kube01

       [docker-node]
       kube03

       # 如果 cni 选择 containerd，则仅需配置 [containerd-master] 和 [containerd-node]
       [containerd-master]
       kube01

       [containerd-node]
       kube03

       [storage]
       kube01
    ```

2. 执行如下命令，进行kubernetes的依赖安装
    ``` bash
    kubez-ansible -i multinode  bootstrap-servers
    ```

3. 执行如下命令，进行kubernetes的集群安装
    ``` bash
    kubez-ansible -i multinode deploy
    ```
