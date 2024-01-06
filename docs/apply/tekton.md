# Tekton 安装

### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)

### 开启 Tekton 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_tekton: "no"` 的注释，并设置为 `"yes"`
    ```shell
    ################
    # Tekton Options
    ###############
    enable_tekton: "yes"

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
    # 查看 Pod 运行状态
    root@VM-0-9-ubuntu:~# kubectl  get pod -A
    NAMESPACE                    NAME                                                 READY   STATUS      RESTARTS   AGE
    tekton-pipelines-resolvers   tekton-pipelines-remote-resolvers-7dd6dddf86-spzqh   1/1     Running     0          16s
    tekton-pipelines             tekton-dashboard-7d74d474f8-dfrxx                    1/1     Running     0          12s
    tekton-pipelines             tekton-pipelines-controller-57d9d77b4-n654v          1/1     Running     0          16s
    tekton-pipelines             tekton-pipelines-webhook-549fd99d48-s6n48            1/1     Running     0          16s
    ```

5. 访问 `Tekton`
    ```shell
    # 获取 Tekton 的 service 信息
    root@VM-0-9-ubuntu:~# kubectl  get svc  -n tekton-pipelines
    NAME                          TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)                              AGE
    tekton-dashboard              ClusterIP   10.254.127.248   <none>        9097/TCP                             2m18s
    tekton-pipelines-controller   ClusterIP   10.254.250.219   <none>        9090/TCP,8008/TCP,8080/TCP           2m23s
    tekton-pipelines-webhook      ClusterIP   10.254.243.71    <none>        9090/TCP,8008/TCP,443/TCP,8080/TCP   2m22s

    # 需要将 tekton-dashboard  手动调整成 NodePort 类型
    # root@VM-0-9-ubuntu:~# kubectl -n tekton-pipelines patch svc tekton-dashboard -p '{"spec":{"type":"NodePort"}}'
    service/tekton-dashboard patched

    # 查看 NodePort 的值
    root@VM-0-9-ubuntu:~# kubectl get svc -n tekton-pipelines  tekton-dashboard
    NAME          TYPE       CLUSTER-IP      EXTERNAL-IP   PORT(S)                         AGE
    tekton-dashboard   NodePort   10.254.127.248   <none>        9097:31959/TCP   5m43s

    # 此时 tekton-dashboard 的访问地址为 公网ip:31959，即可访问到 tekton-dashboard.
    ```

