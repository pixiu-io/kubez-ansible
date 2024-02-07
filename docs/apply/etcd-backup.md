# K8s集群信息备份
本页展示在 Kubernetes 集群中，如何把集群信息备份到本地或(和)符合S3标准的远程存储. 以下是该过程的总结：
1. 需要在安装有kubez-ansible环境下执行, 把globals.yml中的`Etcd-backup Options`注释信息取消, 执行`kubez-ansible apply`或`kubez-ansible -i multinode apply`, 正常情况下会在/var/backups/etcd/下看到备份文件, 默认保留最新的3个文件
2. 备份文件上传到Minio等S3, 需要修改能够连接到服务的地址,密钥等信息
3. 包含一些常见的问题处理方法
### 准备开始
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
### 开始集群备份
1. 取消/etc/kubez/globals.yml中的注释修改为自己的信息
   ```shell
   ##################
   # Etcd-backup Options
   ##################
   #开启备份
   enable_backup: "yes"
   #etcd的运行地址, 修改为实际的etcd地址
   etcd_endpoints: https://172.17.16.13:2379
   #no周期性备份, yes只备份一次
   only_once: "yes"
   #保留备份文件数量
   keep_num: "\"3\""
   #调度周期默认每天凌晨3点
   schedule: `"\"0 3 * * *\""`
   #集群备份镜像地址
   backup_image: tyvek2zhang/etcd-backup:v3.5.11
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
2. 执行安装命令（根据实际情况选择）
   单节点集群场景
   ```shell
   kubez-ansible apply
   ```
   高可用集群场景
   ```shell
   kubez-ansible -i multinode apply
   ```
3. 部署完验证, 分别为备份一次和周期性备份
   ```shell
   kubectl get job -A
   ```
   ```shell
   NAMESPACE      NAME                  COMPLETIONS      DURATION  AGE
   pixiu-system   etcd-backup-once        1/1             3s        7s
   ```
   ```shell
   kubectl get cronjob -A
   ```
   ```shell
   NAMESPACE      NAME                  SCHEDULE      SUSPEND   ACTIVE   LAST SCHEDULE   AGE
   pixiu-system   etcd-backup-regular   *0 3 * * *     False     1        4s             52s
   ```
   可在自定义的备份目录下看到备份文件
   ```shell
   ls /var/backups/etcd/
   ```
   ```shell
   etcd-backup-20240109-03:01:00.db
   ```
4. 常见问题解答
   1. <details>
      <summary>实现原理是什么</summary>
      镜像中包含etcdctl和mc命令行工具, 通过容忍度, 亲和性配置, 使任务在含有etcd证书的节点运行(control plane、master). 此外,参考官网提供的备份, 上传指令实现数据快照生成,以及推送到远程功能
      </details>
   2. <details>
      <summary>备份目录下出现`etcd-backup-*.db.part`结尾的文件</summary>
      检查etcd_endpoints, 以及证书的地址是否正确
      </details>
   3. <details>
      <summary>开启了backup2minio等s3后端, 但是服务器上没有备份文件生成</summary>
      是否有对应的minio等服务启动, 检查参数配置是否正确
      </details>
