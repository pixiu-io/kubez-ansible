# Destory Kubernetes cluster

# 1. 执行如下命令进行kubernetes cluster的清理

    ``` bash
    # multinode
    kubernetes-ansible -i multinode destroy --yes-i-really-really-mean-it

    # all-in-one
    kubernetes-ansible destroy --yes-i-really-really-mean-it
    ```

# 2. 重启服务器或者手动清理残留信息

    ``` bash
    # 残留信息主要有：
    iptables, ipvs, cni等
    ```
