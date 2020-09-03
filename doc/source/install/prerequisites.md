# 准备工作

1. 安装部署节点的依赖,执行

    ``` bash
    curl https://raw.githubusercontent.com/yingjuncao/kubez-ansible/master/tools/setup_env.sh | bash

    如果上述命令因为网络原因执行失败，拷贝`tools/setup_env.sh` 内容到本地，并执行.
    ```

2. (可选，all-in-one场景不需要执行) 编辑当前目录的 `multinode` ，完成主机组配置，手动开通部署节点到工作节点的免密登陆，并用如下命令测试

    ``` bash
    ansible -i multinode all -m ping
    ```
