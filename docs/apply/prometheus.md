# Prometheus

## 部署K8S

- 运行正常的 `kubernetes` ( 1.17+ )环境。安装手册参考 [高可用集群](https://github.com/caoyingjunz/kubez-ansible/blob/master/docs/install/multinode.md) 或 [单节点集群](https://github.com/caoyingjunz/kubez-ansible/blob/master/docs/install/all-in-one.md)

## 开启prometheus组件

1、编辑 `/etc/kubez/globals.yml`

2、取消 `enable_prometheus: "no"` 的注释，并设置为 `"yes"`

```sh
# grafana will also be deploy when prometheus is enable.
enable_prometheus: "yes"
#enable_grafana: "{{ enable_prometheus }}"
```

## 下载资源清单

```sh
git clone https://github.com/iidst/pixiu_ex.git


[root@k8s-151 /etc/kubernetes/pixiu_ex]# ll
total 24
-rw-r--r-- 1 root root  129 Jan 31 20:00 0.pixiu-apiserver
-rw-r--r-- 1 root root  402 Jan 31 20:00 1.dashboard-cluster-admin.yaml
-rw-r--r-- 1 root root  592 Jan 31 20:00 2.dashboard-ingress.yaml
-rw-r--r-- 1 root root 1446 Jan 31 20:00 3.monitoring-ingress.yaml
-rw-r--r-- 1 root root  280 Jan 31 20:00 4.test-ns+pvc.yaml
-rw-r--r-- 1 root root 1655 Jan 31 20:00 5.test-nginx.yaml
```

## 将Prometheus与Grafana暴露在Ingress中

```sh
kubectl apply -f 3.monitoring-ingress.yaml
```

## 配置本地hosts解析并验证

```sh
[root@k8s-151 /etc/kubernetes/pixiu_ex]# kubectl get ing -A
NAMESPACE      NAME             CLASS   HOSTS                   ADDRESS      PORTS   AGE
pixiu-system   alertmanager     nginx   k8s-alertm.pixiu.com    10.0.0.115   80      13h
pixiu-system   grafana          nginx   k8s-grafana.pixiu.com   10.0.0.115   80      13h
pixiu-system   prometheus-k8s   nginx   k8s-prom.pixiu.com      10.0.0.115   80      13h
```

