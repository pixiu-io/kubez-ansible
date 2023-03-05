# Grafana 安装

### 依赖条件
- 运行正常的 `kubernetes` ( v1.21+ )环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)

### 通用模板
- [Node_Exporter](https://grafana.com/grafana/dashboards/1860-node-exporter-full/)
- [Kubernetes ApiServer](https://grafana.com/grafana/dashboards/12006-kubernetes-apiserver/)
- [ETCD](https://grafana.com/grafana/dashboards/3070-etcd/)
- [Nginx Ingress Controller](https://grafana.com/grafana/dashboards/9614-nginx-ingress-controller/)
- [Loki](https://grafana.com/grafana/dashboards/14055-loki-stack-monitoring-promtail-loki/)

### 开启 Grafana 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_grafana: "no"` 的注释，并设置为 `"yes"`
    ```shell
    ##################
    # Grafana Options
    ##################
    enable_grafana: "yes"

    grafana_admin_user: admin
    grafana_admin_password: admin
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
    [root@VM-0-16-centos ~]# kubectl get pod -n pixiu-system
    NAME                      READY   STATUS    RESTARTS   AGE
    grafana-b96497f76-fmrdf   1/1     Running   0          22m

    [root@VM-0-16-centos ~]# helm list -n pixiu-system
    NAME        NAMESPACE       REVISION        UPDATED                                 STATUS          CHART           APP VERSION
    grafana     pixiu-system    1               2023-03-05 17:07:34.860715012 +0800 CST deployed        grafana-6.29.6  8.5.3
