# Dashboard 安装

### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)

### 描述
- 通过dashboard能够直观了解 `kubernetes` 集群中运行的资源对象
- 通过dashboard可以直接管理（创建、删除、重启等操作）资源对象

### 开启dashboard组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_dashboard: "no"` 的注释，并设置为 `"yes"`
    ```shell
    enable_dashboard: "yes"
    dashboard_chart_version: 6.0.0
    ```

3. 执行安装命令（根据实际情况选择）
    ```shell
    # 单节点集群场景
    kubez-ansible apply

    # 高可用集群场景
    kubez-ansible -i multinode apply
    ```

4. 修改 `service` 的服务类型
    ```shell
    [root@master01 ~]# kubectl edit svc -n pixiu-system kubernetes-dashboard
    ...
    spec:
      type: NodePort #此处添加一种访问方式，选用NodePort
      ports:
        - name: https
          port: 443
          targetPort: https
          nodePort: 30666 # 对应 Nodeport，端口范围30000-32767
          protocol: TCP
    ```

5. 添加 `rbac` 的权限
   ```shell
   # 创建用户
   kubectl create serviceaccount dashboard-admin -n pixiu-system

   # 将dashboard-admin用户授cluster-admin权限（clusterrole为集群管理权限）
   kubectl create clusterrolebinding dashboard-admin-rb --clusterrole=cluster-admin --serviceaccount=pixiu-system:dashboard-admin
   ```
6. 部署完验证
   ```shell
   # 查看 pod 状态及暴露的端口
   [root@9eavmhsbs9eghuaa ~]# kubectl get pod,svc -n pixiu-system
   NAME                                       READY   STATUS    RESTARTS   AGE
   pod/helm-toolbox-0                         1/1     Running   0          83m
   pod/kubernetes-dashboard-f8659cff4-h29gz   1/1     Running   0          66m

   NAME                           TYPE       CLUSTER-IP       EXTERNAL-IP   PORT(S)         AGE
   service/kubernetes-dashboard   NodePort   10.254.179.195   <none>        443:30666/TCP   66m
   ```

### 访问 `dashboard`
1. 浏览器访问：`https://<ip>:30666`

2. 页面访问后我们选择 token，回到终端根据下面操作获取 `token`

3. 获取 `token`
   ```shell
   [root@9eavmhsbs9eghuaa ~]# kubectl get secrets -n pixiu-system |grep dashboard-admin
   dashboard-admin-token-p8jlr    kubernetes.io/service-account-token   3      60m
   [root@9eavmhsbs9eghuaa ~]#

    # 获取该 token 密钥进行登录验证
   [root@9eavmhsbs9eghuaa ~]# kubectl describe secrets dashboard-admin-token-p8jlr -n pixiu-system
   Name:         dashboard-admin-token-p8jlr
   Namespace:    pixiu-system
   Labels:       <none>
   Annotations:  kubernetes.io/service-account.name: dashboard-admin
   kubernetes.io/service-account.uid: 579be7f2-23c0-4342-b613-c476668fb89e

   Type:  kubernetes.io/service-account-token

   Data
   ====
   ca.crt:     1099 bytes
   namespace:  12 bytes
   token:      eyJhbGciOiJSUzI1NiIsImtpZCI6ImpWcDZPVXdveWh3Wk5QS014YzI4Y1hrSXgyS1JNbXhrOGQxdWVTcGxMblEifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJwaXhpdS1zeXN0ZW0iLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlY3JldC5uYW1lIjoia3ViZXJuZXRlcy1kYXNoYm9hcmQtdG9rZW4tbHBqZzciLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC5uYW1lIjoia3ViZXJuZXRlcy1kYXNoYm9hcmQiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC51aWQiOiJmMjc2ZDZmYS02ZmVhLTQ5MWQtYmRlOS1kN2EzNzhiOWQwODAiLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6cGl4aXUtc3lzdGVtOmt1YmVybmV0ZXMtZGFzaGJvYXJkIn0.X5M6I8NqJD92191IlSw4-9SbSAkSFF3HeeU9rbKu1rPXnGbqOB0i_pUCE_09FRzSnr1oy8ZRMOXVyUK1IX0KGTkLhqDvsrESDQBzeH9w8-H_DiTTBuS63UPr53pR1Fq7JSUyJ42EEvw71byi2nLYlULmtq7a9dwNbnALBakoGVLuRdPHtdkbmhOj-u4ZfOUfatpDtK3p6zURZFLrAtq0HssiEAE-CYpW5m5pRqm1pxeZKtxKEVB5NRwVJ5j4werj6Ijb8-qRfYLFFKSr3lbYP-Mt1NAw3LwNtBUT1BQEV2bRCwQLHD5G-P8iBHmcO6SA7cqP2mhFZjOlxWfHNqEdLA
   ```
