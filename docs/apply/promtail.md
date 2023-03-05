# Promtail

## 依赖条件
- 运行正常的 `kubernetes` ( v1.21+ )环境。安装手册参考 [高可用集群](https://github.com/gopixiu-io/kubez-ansible/blob/master/docs/install/multinode.md) 或 [单节点集群](https://github.com/gopixiu-io/kubez-ansible/blob/master/docs/install/all-in-one.md)
- Loki

## 开启 Promtail 组件

1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_promtail: "no"` 的注释，并设置为 `"yes"`，loki_url配置为loki的服务端url。
   ```yaml
   ##################
   # promtail Options
   ##################
   enable_promtail: "yes"

   # Loki server url for push
   loki_url: http://loki-gateway/loki/api/v1/push
   ```

3. 执行安装命令（根据实际情况选择）
   ```yaml
   # 单节点集群场景
   kubez-ansible apply

   # 高可用集群场景
   kubez-ansible -i multinode apply
   ```

4. 部署完验证
   ```shell
   [root@VM-16-13-centos ~]# kubectl get pods -n pixiu-system|grep promtail
   promtail-jzg96                                 1/1     Running   0          179m
   promtail-q829g                                 1/1     Running   0          179m
   ```