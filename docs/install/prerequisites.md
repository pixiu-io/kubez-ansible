前置准备工作

1. 安装部署节点的依赖,执行

``` bash
# 方式一、直接安装
#       当有网络的时候建议直接安装(因为简单又方便)
curl https://raw.githubusercontent.com/caoyingjunz/kubez-ansible/master/tools/setup_env.sh | bash

# 方式二、如果网络不通或者没有curl命令，则需要手动将 tools/setup_env.sh拷贝到本地，并执行
#       (可选)不管网络通不通都可以使用下面这个步骤，如果网络是通的，上面(方式一)直接安装最适合你，方便既快捷
bash tools/setup_env.sh
```

2. (可选，all-in-one场景不需要执行) 编辑当前目录的 `multinode` ，完成主机组配置，手动开通部署节点到工作节点的免密登陆，并用如下命令测试

    ``` bash
    ansible -i multinode all -m ping
    ```

