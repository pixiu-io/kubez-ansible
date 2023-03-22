# 生成kubernetes RC 文件

1. 完成 `kubernetes` 的部署之后，需要导入KUBECONFIG到环境变量
    ``` bash
    # multinode
    kubez-ansible -i multinode post-deploy

    # all-in-one
    kubez-ansible post-deploy
    ```

2. 验证, 得到类似回显
    ``` bash
    . /root/admin-k8src.sh

    kubectl get node
    ```
