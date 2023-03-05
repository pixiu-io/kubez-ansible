# Istio

## 依赖条件

- 运行正常的 `kubernetes` ( v1.21+ )环境。安装手册参考 [高可用集群](https://github.com/gopixiu-io/kubez-ansible/blob/master/docs/install/multinode.md) 或 [单节点集群](https://github.com/gopixiu-io/kubez-ansible/blob/master/docs/install/all-in-one.md)

## 开启 Istio组件

1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_istio: "no"` 的注释，并设置为 `"yes"`，设置istio的版本`istio_chart_version: "1.16.1"`

   ```yaml
   ################
   # Istio Options
   ################
   enable_istio: "yes"
   istio_chart_version: "1.16.1"
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
   [root@VM-16-11-centos ~]# kubectl get pods -n istio-system
   NAME                      READY   STATUS    RESTARTS   AGE
   istiod-5b86c45f48-mjzsd   1/1     Running   0          2m49s
   ```