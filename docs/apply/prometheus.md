# Prometheus

## 部署K8S

- 运行正常的 `kubernetes` ( 1.17+ )环境。安装手册参考 [高可用集群](https://github.com/caoyingjunz/kubez-ansible/blob/master/docs/install/multinode.md) 或 [单节点集群](https://github.com/caoyingjunz/kubez-ansible/blob/master/docs/install/all-in-one.md)

## 开启prometheus组件

1、编辑 `/etc/kubez/globals.yml`

2、取消 `enable_prometheus: "no"` 的注释，并设置为 `"yes"`

```sh
# grafana will also be deploy when prometheus is enable.
enable_prometheus: "yes"
```

## 下载资源清单

```sh
# 使用以下命令获取grafana密码
#  kubectl get secret -n pixiu-system  grafana -o yaml |grep password | awk '{ print $2 }'  | base64 -d
#
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  labels:
    app: grafana
  name: grafana
  namespace: pixiu-system
spec:
  ingressClassName: nginx
  rules:
    - host: k8s-grafana.pixiu.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: grafana
                port:
                  number: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  labels:
    app: prometheus
    prometheus: k8s
  name: prometheus-k8s
  namespace: pixiu-system
spec:
  ingressClassName: nginx
  rules:
    - host: k8s-prom.pixiu.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: prometheus-server
                port:
                  number: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  labels:
    app: alertmanager
    prometheus: k8s
  name: alertmanager
  namespace: pixiu-system
spec:
  ingressClassName: nginx
  rules:
    - host: k8s-alertm.pixiu.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: prometheus-alertmanager
                port:
                  number: 80
```

## 将Prometheus与Grafana暴露在Ingress中

```sh
kubectl apply -f 3.monitoring-ingress.yaml
```

## 配置本地hosts解析并验证

```sh
[root@k8s-151 ~]# kubectl get ing -A
NAMESPACE      NAME             CLASS   HOSTS                   ADDRESS      PORTS   AGE
pixiu-system   alertmanager     nginx   k8s-alertm.pixiu.com    10.0.0.115   80      13h
pixiu-system   grafana          nginx   k8s-grafana.pixiu.com   10.0.0.115   80      13h
pixiu-system   prometheus-k8s   nginx   k8s-prom.pixiu.com      10.0.0.115   80      13h
```

