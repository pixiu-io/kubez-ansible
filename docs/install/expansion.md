# 集群扩容

1. 配置工作目录下的 `multinode` , 根据实际情况添加待扩容节点到 `cri` 组, 并完成如下配置
  - 打通部署节点到新增节点的免密登陆 [批量开启免密登陆](auth-key.md)

  - 配置部署节点的 `/etc/hosts`, 添加新增节点的ip和主机名解析(本例以新增 `kube03` 为例), `multinode` 配置格式
    - 如果 `cri` 选择 `docker`，
        ```shell
        # 仅需配置 [docker-master] 和 [docker-node] 分组
        [docker-master]
        kube01

        [docker-node]
        kube03
        ```
    - 如果 `cri` 选择 `containerd`
        ```shell
        # 仅需配置 [containerd-master] 和 [containerd-node]
        [containerd-master]
        kube01

        [containerd-node]
        kube03
        ```

2. 执行如下命令，进行kubernetes的依赖安装
    ``` bash
    kubez-ansible -i multinode  bootstrap-servers
    ```

3. 执行如下命令，进行kubernetes的集群安装
    ``` bash
    kubez-ansible -i multinode deploy
    ```
