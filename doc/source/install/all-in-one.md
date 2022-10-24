# 单节点部署 -- All-in-one

1. 部署前准备--[前提条件](prerequisites.md)

2. 检查虚拟机默认网卡配置:

   a. 默认网卡为 `eth0`, 如果环境实际网卡不是 `eth0`，则需要手动指定网卡名称:

   ``` bash
   编辑 /etc/kubez/globals.yml 文件，取消 network_interface: "eth0" 的注解，并修改为实际网卡名称
   ```

3. 确认集群环境连接地址:

   a. 内网连接: 无需更改

   b. 公网地址:
   ```bash
      编辑 /etc/kubez/globals.yml 文件，取消 #kube_vip_address: "172.16.50.250" 与 #kube_vip_port: 8443 的注解
      修改成实际地址与端口
   ```
4. all-in-one 环境的自定义安装

    a. 执行如下命令，进行kubernetes的依赖安装

    ``` bash
    kubez-ansible bootstrap-servers
    ```

    b.（可选）使用离线安装模式 -- [开启本地私有仓库](setup-registry.md)

    c. 执行如下命令，进行 `kubernetes` 的依赖安装

    ``` bash
    kubez-ansible deploy
    ```

5. 生成 kubernetes RC 文件 `.kube/config`
   ``` bash
   kubez-ansible post-deploy
   ```

6. 验证环境
   ```bash
   # kubectl get node
   NAME    STATUS   ROLES    AGE    VERSION
   kubez   Ready    master   134d   v1.23.6
   ```
