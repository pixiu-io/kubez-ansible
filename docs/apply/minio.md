# MinIO 安装
### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)

### 开启 Minio 组件
1. 开启 `minio` 并进行参数配置
    ```shell
   # 取消 `enable_minio: "no" 的注释, 并设置为 "yes" 可开启 minio
   #################
   # Minio Options
   #################
   enable_minio: "yes"
   #minio_namespace: "{{ kubez_namespace }}"

   # 取消注释并按照实际情况修改
   #minio_storage_class: managed-nfs-storage
   #minio_storage_size: 500Gi
   # 生产环境推荐 16Gi
   #minio_memory_size: 4Gi

   # applicable only for MinIO distributed mod
   # 定义副本数量，生产环境推荐 16
   #minio_replicas: 4
   # 定义 Web 界面登录的用户以及密码
   #minio_rootUser: minioadmin
   #minio_rootPassword: minioadmin
    ```
2. 执行安装命令（根据实际情况选择）

   ```shell
   # 单节点集群场景
   kubez-ansible apply

   # 高可用集群场景
   kubez-ansible -i multinode apply
   ```
3. 部署完验证
   ```shell
   [root@VM-10-centos ~]# kubectl get pods -n pixiu-system|grep minio
   NAMESPACE       NAME                                  READY   STATUS    RESTARTS         AGE
   pixiu-system    minio-0                               1/1     Running     0              168m
   pixiu-system    minio-1                               1/1     Running     0              168m
   pixiu-system    minio-2                               1/1     Running     0              168m
   pixiu-system    minio-3                               1/1     Running     0              168m
   ```
4. （可选）修改 `svc` 验证 `Web` 界面登录访问
   ```shell
   [root@VM-10-centos ~]#kubectl get svc -A  | grep  minio
   minio               minio                            ClusterIP      10.43.185.7     <none>        9000/TCP                5d4h
   minio               minio-console                    ClusterIP      10.43.45.84     <none>        9001/TCP                5d4h
   minio               minio-svc                        ClusterIP      None            <none>        9000/TCP                5d4h
   [root@VM-10-centos ~]# kubectl edit svc -n pixiu-system minio-console
   将 `type: ClusterIP` 修改为 `type: NodePort`
   ```
   然后再查看 `svc`
   ```shell
   [root@VM-10-centos ~]#kubectl get svc -A  | grep  minio
   minio               minio                            ClusterIP      10.43.185.7     <none>        9000/TCP                5d4h
   minio               minio-console                    NodePort       10.43.45.84     <none>        9001:30485/TCP          5d4h
   minio               minio-svc                        ClusterIP      None            <none>        9000/TCP                5d4h
   ```
   在浏览器输入服务器 `IP+port` (如果是云主机保证端口在安全组放开)
