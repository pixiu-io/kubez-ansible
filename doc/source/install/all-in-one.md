# 测试环境部署 -- All-in-one

1. (可选) All-in-one环境的快速安装 - 配置全部默认

    ``` bash
    curl https://raw.githubusercontent.com/yingjuncao/kubernetes-ansible/master/tools/all_in_one.sh | bash

    如果上述命令因为网络原因执行失败，拷贝 `tools/all_in_one.sh` 内容到本地，并执行.
    ```

2. All-in-one环境的自定义安装

    a. 部署前准备--[前提条件](prerequisites.md)

    b. 执行如下命令，进行kubernetes的依赖安装

    ``` bash
    kubernetes-ansible bootstrap-servers
    ```

    c.（可选）使用离线安装模式 --[开启本地私有仓库](setup-registry.md)

    d. 执行如下命令，进行kubernetes的依赖安装

    ``` bash
    kubernetes-ansible deploy
    ```
