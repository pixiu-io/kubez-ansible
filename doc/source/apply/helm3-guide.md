# Helm3 guide

1. 配置 `/etc/kubernetes-ansible/globals.yml`, 开启helm选项（默认关闭）

    ``` bash
    enable_helm: "yes"
    ```

2. 执行如下命令完成 `helm3` 的安装.

    ``` bash
    # multinode
    kubernetes-ansible -i multinode apply

    # all-in-one
    kubernetes-ansible apply

   （可选）：直接拷贝 `tools/setup_helm.sh`到指定节点并执行, 完成helm的安装.
    ```

3. 验证, 得到类似回显

    ``` bash
    export KUBECONFIG=/etc/kubernetes/admin.conf

    helm version
    version.BuildInfo{Version:"v3.0.3", GitCommit:"ac925eb7279f4a6955df663a0128044a8a6b7593", GitTreeState:"clean", GoVersion:"go1.13.6"}
    ```
