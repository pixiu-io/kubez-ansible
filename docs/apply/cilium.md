# Cilium & Hubble

## 依赖条件

- 运行正常的 `kubernetes` ( v1.21+ )环境。安装手册参考 [高可用集群](https://github.com/gopixiu-io/kubez-ansible/blob/master/docs/install/multinode.md) 或 [单节点集群](https://github.com/gopixiu-io/kubez-ansible/blob/master/docs/install/all-in-one.md)
- Linux kernel >= 4.9.17

## 开启 Cilium 组件

1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_cilium: "no"` 的注释，并设置为 `"yes"`，设置Cilium的版本 `cilium_chart_version: "1.16.1"`

   ```yaml
   ################
   # Cilium Options
   ################
   enable_cilium: "yes"
   cilium_chart_version: "1.14.5"
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
   [root@VM-16-11-centos ~]# kubectl get pods -n kube-system
   NAME                                    READY   STATUS    RESTARTS   AGE
   cilium-operator-cb4578bc5-q52qk         1/1     Running   0          4m13s
   cilium-s8w5m                            1/1     Running   0          4m12s
   ```
