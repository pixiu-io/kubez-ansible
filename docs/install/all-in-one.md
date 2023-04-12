# 单节点集群

### 系统要求
- `1C2G+`

### 依赖条件
- [依赖安装](prerequisites.md)

### 部署步骤
1. 检查虚拟机默认网卡配置
   - 默认网卡为 `eth0`, 如果环境实际网卡不是 `eth0`，则需要手动指定网卡名称:
     ```bash
     编辑 /etc/kubez/globals.yml 文件, 取消 network_interface: "eth0" 的注解, 并修改为实际网卡名称
     ```

2. 确认集群环境连接地址

   a. 内网连接: 无需更改

   b. 公网连接:
   ```bash
   编辑 /etc/kubez/globals.yml 文件, 取消 #kube_vip_address: "" 的注解，并修改为实际公网地址 云平台环境需要放通公网ip到后面节点的6443端口
   ```

3. (可选) 修改默认的 `cri`
- 默认的 `cri` 为 `containerd`, 如果期望修改为 `docker`, 则
  - `Centos` 修改 `/usr/share/kubez-ansible/ansible/inventory/all-in-one`
  - `Ubuntu` 修改 `/usr/local/share/kubez-ansible/ansible/inventory/all-in-one`

- 移除 `containerd-master` 和 `containerd-node` 的主机信息, 并添加在 `docker` 分组中, 调整后效果如下:
  ```shell
  [docker-master]
  localhost       ansible_connection=local

  [docker-node]
  localhost       ansible_connection=local

  [containerd-master]

  [containerd-node]
  ```

4. 执行如下命令，进行 `kubernetes` 的依赖安装
    ```bash
    kubez-ansible bootstrap-servers
    ```

5. 执行如下命令，进行 `kubernetes` 的集群安装
    ``` bash
    kubez-ansible deploy
    ```

6. 验证环境
   ```bash
   # kubectl get node
   NAME    STATUS   ROLES    AGE    VERSION
   pixiu   Ready    master   134d   v1.23.6
   ```
7. (可选)启用 kubectl 命令行补全
    ``` bash
    kubez-ansible post-deploy
    ```
