# Loggie

## 依赖条件
- 运行正常的 `kubernetes` ( v1.21+ )环境。安装手册参考 [高可用集群](https://github.com/gopixiu-io/kubez-ansible/blob/master/docs/install/multinode.md) 或 [单节点集群](https://github.com/gopixiu-io/kubez-ansible/blob/master/docs/install/all-in-one.md)
- Loki

## 开启 Loggie 组件

1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_loggie: "no"` 的注释，并设置为 `"yes"`，`loggie_loki_url` 配置为 `loki` 的服务端 `url`。
   ```yaml
    #################
    # Loggie Options
    #################
    enable_loggie: "yes"
    #loggie_namespace: "{{ kubez_namespace }}"

    #loggie_loki_url: http://loki-gateway/loki/api/v1/push
    # helm 仓库配置项
    #loggie_repo_name: "{{ default_repo_name }}"
    #loggie_repo_url: "{{ default_repo_url }}"
    #loggie_path: pixiuio/loggie
    #loggie_version: 1.4.0
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
    root@ubuntu:~# kubectl  get  pod  -A | grep  loggie
    pixiu-system   loggie-g484w                                         1/1     Running     0             17h
   ```
