# Ingress Nginx 安装

### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)

### 安装 Ingress Nginx组件
1. 编辑 `/etc/kubez/globals.yml`

2. 配置是默认安装，如果不安装，则取消 `enable_ingress_nginx: "yes"` 的注释，并设置为 `"no"`
    - 注意：安装好再设置成no，再执行第3步安装命令是无效的
    ```shell
    #######################
    # Ingress Nginx Options
    #######################
    enable_ingress_nginx: "yes"
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
    # 所有的 `ingress pod` 均运行正常
    [root@VM-32-9-centos ~]# kubectl get pod -n kube-system
    NAMESPACE       NAME                                        READY   STATUS      RESTARTS      AGE
    kube-system     ingress-nginx-admission-create-kvrkq        0/1     Completed   0             4d3h
    kube-system     ingress-nginx-admission-patch-999z9         0/1     Completed   5             4d3h
    kube-system     ingress-nginx-controller-58c95c57d4-lklsj   1/1     Running     2 (17h ago)   4d2h
    ```

5. (可选) 设置宿主机的 `ip` 作为 `ingress` 的入口 —— 适用于没有 `LB` 又想用 `Ingress` 场景
    - 编辑 `/tmp/pixiuspace/ingress-nginx.yml` 加上 `hostNetwork:true` 配置
    ```shell
        ...
        spec:
          ...
          hostNetwork: true
          ...
    ```
    - 生效配置文件
    ```shell
       kubectl apply -f /tmp/pixiuspace/ingress-nginx.yml -n kube-system
    ```
