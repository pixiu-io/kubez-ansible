# Destroy cluster

1. 执行如下命令进行kubernetes cluster的清理
    ``` bash
    # multinode
    kubez-ansible -i multinode destroy --yes-i-really-really-mean-it

    # all-in-one
    kubez-ansible destroy --yes-i-really-really-mean-it
    ```

2. 重启服务器或者手动清理残留信息
    ``` bash
    # 残留信息主要有：
    iptables, ipvs, cni 等
    ```
