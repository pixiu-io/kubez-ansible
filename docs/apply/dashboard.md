# Dashboard安装

`通过dashboard能够直观了解Kubernetes集群中运行的资源对象`

`通过dashboard可以直接管理（创建、删除、重启等操作）资源对象`
#### 开启dashboard组件
```shell
1. 编辑 `/etc/kubez/globals.yml`
2. 取消 `#enable_dashboard: "no"` 的注释，并设置为 `"yes"`
enable_dashboard: "yes"
dashboard_chart_version: 6.0.0
```
#### 修改dashboard资源清单文件
~~~shell
[root@master01 ~]# kubectl edit svc -n pixiu-system kubernetes-dashboard
......
1.找到spec部分24行左右,添加NodePort类型及端口
---
spec:
  type: NodePort         #此处添加一种访问方式，选用NodePort
  ports:
    - name: https
      port: 443
      targetPort: 6443
      nodePort: 30666    #对应Nodeport，端口范围30000-32767
      protocol: TCP
2.查看ns
[root@VM-0-16-centos tmp]# kubectl get ns|grep pixiu-system
pixiu-system      Active   115m
3.查看pod状态及暴露的端口
[root@VM-0-16-centos tmp]# kubectl get pod,svc -n pixiu-system
NAME                                       READY   STATUS    RESTARTS       AGE
pod/helm-toolbox-0                         1/1     Running   0              114m
pod/kubernetes-dashboard-f8659cff4-494rv   1/1     Running   17 (15m ago)   89m

NAME                           TYPE       CLUSTER-IP     EXTERNAL-IP   PORT(S)         AGE
service/kubernetes-dashboard   NodePort   10.254.9.252   <none>        443:30666/TCP   89m 
~~~
#### 访问dashboard

1.`浏览器访问：https://1.13.190.153:30666/`

2.`页面访问后我们选择token，回到终端根据下面操作获取token`

3.`开始获取token`
~~~shell
[root@VM-0-16-centos tmp]# kubectl get secret -n pixiu-system
NAME                                         TYPE                                  DATA   AGE
default-token-q2ztw                          kubernetes.io/service-account-token   3      109m
kubernetes-dashboard-certs                   Opaque                                0      83m
kubernetes-dashboard-csrf                    Opaque                                1      83m
kubernetes-dashboard-key-holder              Opaque                                2      83m
kubernetes-dashboard-token-lpjg7             kubernetes.io/service-account-token   3      83m
sh.helm.release.v1.kubernetes-dashboard.v1   helm.sh/release.v1                    1      83m
...
获取该token密钥进行登录验证


获取token密钥
[root@VM-0-16-centos tmp]# kubectl describe secret kubernetes-dashboard-token-lpjg7 -n pixiu-system
Name:         kubernetes-dashboard-token-lpjg7
Namespace:    pixiu-system
Labels:       <none>
Annotations:  kubernetes.io/service-account.name: kubernetes-dashboard
              kubernetes.io/service-account.uid: f276d6fa-6fea-491d-bde9-d7a378b9d080

Type:  kubernetes.io/service-account-token

Data
====
ca.crt:     1099 bytes
namespace:  12 bytes
token:      eyJhbGciOiJSUzI1NiIsImtpZCI6ImpWcDZPVXdveWh3Wk5QS014YzI4Y1hrSXgyS1JNbXhrOGQxdWVTcGxMblEifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJwaXhpdS1zeXN0ZW0iLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlY3JldC5uYW1lIjoia3ViZXJuZXRlcy1kYXNoYm9hcmQtdG9rZW4tbHBqZzciLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC5uYW1lIjoia3ViZXJuZXRlcy1kYXNoYm9hcmQiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC51aWQiOiJmMjc2ZDZmYS02ZmVhLTQ5MWQtYmRlOS1kN2EzNzhiOWQwODAiLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6cGl4aXUtc3lzdGVtOmt1YmVybmV0ZXMtZGFzaGJvYXJkIn0.X5M6I8NqJD92191IlSw4-9SbSAkSFF3HeeU9rbKu1rPXnGbqOB0i_pUCE_09FRzSnr1oy8ZRMOXVyUK1IX0KGTkLhqDvsrESDQBzeH9w8-H_DiTTBuS63UPr53pR1Fq7JSUyJ42EEvw71byi2nLYlULmtq7a9dwNbnALBakoGVLuRdPHtdkbmhOj-u4ZfOUfatpDtK3p6zURZFLrAtq0HssiEAE-CYpW5m5pRqm1pxeZKtxKEVB5NRwVJ5j4werj6Ijb8-qRfYLFFKSr3lbYP-Mt1NAw3LwNtBUT1BQEV2bRCwQLHD5G-P8iBHmcO6SA7cqP2mhFZjOlxWfHNqEdLA
~~~

