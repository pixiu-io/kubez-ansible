# Loki

## 依赖条件
- 运行正常的 `kubernetes` ( v1.21+ )环境。安装手册参考 [高可用集群](https://github.com/gopixiu-io/kubez-ansible/blob/master/docs/install/multinode.md) 或 [单节点集群](https://github.com/gopixiu-io/kubez-ansible/blob/master/docs/install/all-in-one.md)
- StorageClass
- Minio（创建三个bucket，分别为chunks，ruler，admin）

## 开启 Loki 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_loki: "no"` 的注释，并设置为 `"yes"`，S3的相关参数需要根据自己的环境进行修改。
   ```yaml
   ##############
   # Loki Options
   ##############
   enable_loki: "yes"

   # Storage class to be used
   loki_storage_class: managed-nfs-storage
   # Size of persistent disk
   loki_storage_size: 10Gi
   # Should authentication be enabled
   loki_auth_enabled: 'false'

   # Number of replicas
   loki_commonConfig_replication_factor_number: 2
   # Number of customized loki read-write replicas
   loki_read_replicas_number: 2
   loki_write_replicas_number: 2

   # Storage config
   loki_storage_bucketNames_chunks: chunks
   loki_storage_bucketNames_ruler: ruler
   loki_storage_bucketNames_admin: admin

   # S3 配置
   s3:
     endpoint: http://172.17.16.13:9000
     secretAccessKey: minioadmin
     accessKeyId: minioadmin
     s3ForcePathStyle: true
     insecure: true
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
   # 所有的 loki pod 均运行正常
   [root@VM-16-13-centos ~]# kubectl get pods -n pixiu-system|grep loki
   loki-canary-c4twh                              1/1     Running   0          3h12m
   loki-canary-j7xsx                              1/1     Running   0          3h12m
   loki-gateway-7c76bc95dd-6s8g4                  1/1     Running   0          3h12m
   loki-grafana-agent-operator-5798474874-p6sz6   1/1     Running   0          3h12m
   loki-logs-4c58m                                2/2     Running   0          3h12m
   loki-logs-gjkm2                                2/2     Running   0          3h12m
   loki-read-0                                    1/1     Running   0          3h12m
   loki-read-1                                    1/1     Running   0          3h12m
   loki-write-0                                   1/1     Running   0          3h12m
   loki-write-1                                   1/1     Running   0          3h12m
   ```