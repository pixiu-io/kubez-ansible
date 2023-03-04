# Helm3 guide

1. 配置 `/etc/kubez/globals.yml`, 开启helm选项（默认关闭）
    ``` bash
    enable_helm: "yes"
    ```

2. 执行如下命令完成 `helm3` 的安装.
    ``` bash
    # multinode
    kubez-ansible -i multinode apply

    # all-in-one
    kubez-ansible apply
    ```

3. 验证, 得到类似回显
    ``` bash
    export KUBECONFIG=/etc/kubernetes/admin.conf

    helm version
    version.BuildInfo{Version:"v3.5.2", GitCommit:"167aac70832d3a384f65f9745335e9fb40169dc2", GitTreeState:"dirty", GoVersion:"go1.15.7"}
    ```
