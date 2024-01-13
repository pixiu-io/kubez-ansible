# K8s集群信息备份
本页展示在 Kubernetes 集群中，如何把集群信息备份到本地,或(和)符合S3标准的远程存储.以下是该过程的总结：
1. 在安装kubez-ansible中执行`kubez-ansible etcd-backup`或`kubez-ansible -i multinode etcd-backup`, 正常情况下会在/var/backups/etcd/下看到备份文件
2. 周期性备份和上传到Minio等S3, 需要修改能够连接到服务的地址,密钥等信息
3. 包含一些常见的问题处理方法
### 准备开始
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
### 开始集群备份
1. 本地备份, 直接可跳到第2条.
2. 周期性备份, 或(和)自动上传到Minio等S3, 取消/etc/kubez/globals.yml中的注释修改为自己的信息
   ```shell
   ##################
   # Etcd-backup Options
   ##################
   #非周期性备份
   only_once: "no"
   #保留备份文件数量
   keep_num: "\"3\""
   #调度周期默认每天凌晨3点
   schedule: `"\"0 3 * * *\""`
   #集群备份镜像地址
   backup_image: tyvek2zhang/etcd-backup:v3.5.11
   #etcd的运行地址, aio默认时候本机ip, 多节点默认kube-master组中第一个节点
   etcd_endpoints: https://localhost:2379
   #etcd_pki的目录
   etcd_pki_dir: /etc/kubernetes/pki/etcd
   #etcd证书的cacert
   etcd_cacert: "{{ etcd_pki_dir }}/ca.crt"
   #etcd证书的cert
   etcd_cert: "{{ etcd_pki_dir }}/server.crt"
   #etcd证书的key
   etcd_key: "{{ etcd_pki_dir }}/server.key"
   #是否开启minio等s3存储, 开启改为"1", 开启后必须配置s3信息
   backup2minio: "\"0\""
   #备份数据使用的桶名称
   s3_bucket: etcd-backups
   #备份的路径
   backup_dir: /var/backups
   #镜像的默认时区
   tz: Asia/Shanghai
   # S3 config
   s3:
     #minio等s3的服务地址
     endpoint: http://172.17.16.13:9000
     #secretKey用于构建客户端
     secretAccessKey: minioadmin
     #accessKey用于构建客户端
     accessKeyId: minioadmin
     s3ForcePathStyle: true
     insecure: true
   ```
3. 执行安装命令（根据实际情况选择）
   单节点集群场景
   ```shell
   kubez-ansible etcd-backup
   ```
   高可用集群场景
   ```shell
   kubez-ansible -i multinode etcd-backup
   ```
4. 部署完验证, 以周期性备份为例, 默认是凌晨3点执行, 查看结果需要在执行时间之后
   ```shell
   kubectl get cronjob -A
   ```
   ```shell
   NAMESPACE      NAME                  SCHEDULE      SUSPEND   ACTIVE   LAST SCHEDULE   AGE
   pixiu-system   etcd-backup-regular   *0 3 * * *     False     1        4s             52s
   ```
   到调度时间后, 可在自定义的备份目录下看到备份文件
   ```shell
   ls /var/backups/etcd/
   ```
   ```shell
   etcd-backup-20240109-03:01:00.db
   ```
5. 常见问题解答
   1. 启动的pod默认调度到哪里
      - master, control-plane节点
   2. 为什么查看pod的状态是error状态?
      - 通过`kubectl logs etcd-backup-regular-xxxx -n pixiu-system`查看失败原因
   3. 我们集群的etcd版本是v2版本的是否可用此方法进行备份?
      - 暂时支持v3版本, 有需要的话可联系作者
   4. 备份目录下出现`etcd-backup-*.db.part`结尾的文件
      - 检查etcd_endpoints, 以及证书的地址是否正确
   5. 开启了backup2minio等s3后端, 但是没有备份文件生成
      - 是否有对应的minio等服务启动, 检查参数配置是否正确
