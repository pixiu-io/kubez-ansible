# k8s集群备份配置
### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)

### 开启集群备份按钮
1. 开启 `enable_backup` 并进行参数配置
    ```shell
   #根据实际情况改为自己的
   ##################
   # Etcd-backup Options
   ##################
   #开启备份
   enable_backup: "yes"
   #保留备份文件数量
   keep_num: "\"3\""
   #调度周期(凌晨3点),测试时使用*/1 * * * *(每分钟)
   schedule: "\"0 3 * * *\""
   #集群备份镜像地址
   backup_image: tyvek2zhang/etcd-backup:v3.5.11
   #etcd的运行地址
   etcd_endpoints: https://172.17.16.13:2379
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

   ```shell
   # 单节点集群场景
   kubez-ansible apply

   # 高可用集群场景
   kubez-ansible -i multinode apply
   ```
   
3. 部署完验证
   
   ```shell
   kubectl get cronjob -A
   ```
   
   ```shell
   NAMESPACE      NAME                  SCHEDULE      SUSPEND   ACTIVE   LAST SCHEDULE   AGE
   pixiu-system   etcd-backup-regular   */1 * * * *   False     1        4s              52s
   ```
   
   以测试一分钟为例, 可在自定义的备份目录下看到备份文件
   
   ```shell
   ls /var/backups/etcd/
   ```
   
   ```shell
   etcd-backup-20240109-14:17:00.db  etcd-backup-20240109-14:18:00.db  etcd-backup-20240109-14:19:00.db
   ```

4. 常见问题解答
   1. 为什么查看pod的状态是error状态?
      - 通过`kubectl logs etcd-backup-regular-xxxx -n pixiu-system`查看失败原因
   2. 我们集群的etcd版本是v2版本的是否可用此方法进行备份?
      - 暂时支持v3版本, 有需要的话可联系作者
   3. 备份目录下出现`etcd-backup-*.db.part`结尾的文件
      - 检查etcd_endpoints, 以及证书的地址是否正确
   4. 开启了backup2minio等s3后端, 但是没有备份文件生成
      - 是否有对应的minio等服务启动, 检查参数配置是否正确