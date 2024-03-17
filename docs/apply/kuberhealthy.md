# Kuberhealthy 安装
### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)

### 开启 Kuberhealthy 组件
1. 开启 `kuberhealthy` 并进行参数配置

   ```shell
   # 取消 `enable_kuberhealthy: "no" 的注释, 并设置为 "yes" 可开启 kuberhealthy
   ###############
   # Kuberhealthy Options
   ###############
   # https://github.com/kuberhealthy/kuberhealthy
   enable_kuberhealthy: "yes"
   kuberhealthy_namespace: pixiu-system
   kuberhealthy_version: "100" # 安装版本，可通过helm search repo kuberhealthy/kuberhealthy --versions查看当前支持版本
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
   $ kubectl get pods -n pixiu-system | grep kuberhealthy
   NAMESPACE       NAME                                  READY   STATUS    RESTARTS         AGE
   kuberhealthy-5879d576d8-lmx7x                     1/1     Running             0          2m47s
   kuberhealthy-5879d576d8-sm55t                     0/1     Pending             0          2m47s
   ```

4. 访问kuberhealthy

   ```shell
   # 访问svc查看结果
   $ kubectl get svc kuberhealthy -n pixiu-system -o=jsonpath='{.spec.clusterIP}' | xargs -I {} curl http://{}/
   ```
