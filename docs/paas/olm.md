# OLM 安装

### 依赖条件
- 运行正常的 `kubernetes` ( v1.17+ )环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)

### 开启 OLM 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_olm: "no"` 的注释，并设置为 `"yes"`
    ```shell
    ####################################
    # Operator-Lifecycle-Manager Options
    ####################################
    enable_olm: "yes"
    ```

3. 执行安装命令（根据实际情况选择）
    ```shell
    # 单节点集群场景
    kubez-ansible apply

    # 高可用集群场景
    kubez-ansible -i multinode apply
    ```

4. 部署完验证
    ```shell
    # 所有的 olm pod 均运行正常
    [root@VM-32-9-centos ~]# kubectl get pod -n olm
    NAME                                READY   STATUS        RESTARTS   AGE
    catalog-operator-755d759b4b-lwhjz   1/1     Running       0          2m6s
    olm-operator-c755654d4-br2qz        1/1     Running       0          2m6s
    operatorhubio-catalog-lzccq         1/1     Running       0          91s
    packageserver-599f7fb5fd-x5w4s      1/1     Running       0          94s
    packageserver-599f7fb5fd-zkrkx      1/1     Running       0          94s
    ```
