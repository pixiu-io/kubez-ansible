# Prometheus Operator

## 部署K8S

- 运行正常的 `kubernetes` ( 1.17+ )环境。安装手册参考 [高可用集群](https://github.com/caoyingjunz/kubez-ansible/blob/master/docs/install/multinode.md) 或 [单节点集群](https://github.com/caoyingjunz/kubez-ansible/blob/master/docs/install/all-in-one.md)

## 开启prometheus Operator组件
1. 开启 `prometheus operator` 并进行参数配置

编辑 `/etc/kubez/globals.yml`
取消 `enable_kube_prometheus_stack: "no"` 的注释，并设置为 `"yes"`

2. 执行安装命令（根据实际情况选择）

   ```shell
   # 单节点集群场景
   kubez-ansible apply

   # 高可用集群场景
   kubez-ansible -i multinode apply
   ```

## 验证
```sh
[root@k8s-151 ~]# kubectl get pods -A | grep prometheus
alertmanager-kube-prometheus-stack-alertmanager-0           2/2     Running   0          13m
kube-prometheus-stack-grafana-0                             3/3     Running   0          13m
kube-prometheus-stack-kube-state-metrics-55555dfcf7-psth8   1/1     Running   0          13m
kube-prometheus-stack-operator-dc85d4f84-wjjmv              1/1     Running   0          13m
kube-prometheus-stack-prometheus-node-exporter-xrmcs        1/1     Running   0          13m
prometheus-kube-prometheus-stack-prometheus-0               2/2     Running   0          13m
```
