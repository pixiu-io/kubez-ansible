# Ingress Nginx 安装

### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)

### 安装 Ingress Nginx组件
1. 编辑 `/etc/kubez/globals.yml`

2. 配置是默认安装，如果不安装，取消 `enable_ingress_nginx: "yes"` 的注释，并设置为 `"no"`
    ```shell
    ################
    # Ingress Nginx Options
    ################
    #enable_ingress_nginx: "yes"
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
    # 所有的 ingress pod 均运行正常
    [root@VM-32-9-centos ~]# kubectl get pod -n kube-system
    NAMESPACE       NAME                                        READY   STATUS      RESTARTS      AGE
    kube-system     ingress-nginx-admission-create-kvrkq        0/1     Completed   0             4d3h
    kube-system     ingress-nginx-admission-patch-999z9         0/1     Completed   5             4d3h
    kube-system     ingress-nginx-controller-58c95c57d4-lklsj   1/1     Running     2 (17h ago)   4d2h
    
    ```
