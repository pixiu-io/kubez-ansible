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

3. 确认默认 CRI

   Kubernetes CRI 仅支持 `containerd`（docker 作为 CRI 已移除）。默认 `all-in-one` inventory 已将主机放在 `[containerd-master]` / `[containerd-node]`；请勿再向 `[docker-master]` / `[docker-node]` 添加主机。

4. (可选)修改 kubernetes 镜像仓库
    ``` bash
    编辑 /etc/kubez/globals.yml 文件，修改 image_repository: "" 为期望镜像仓库，默认是阿里云 registry.cn-hangzhou.aliyuncs.com/google_containers
    ```

5. (可选)修改基础应用镜像仓库

   ```bash
    编辑 /etc/kubez/globals.yml 文件，修改 app_image_repository: "" 为期望镜像仓库，默认是 pixiu镜像仓库 ccr.ccs.tencentyun.com/pixiucloud
    ```

6. 执行如下命令，进行 `kubernetes` 的依赖安装
    ```bash
    kubez-ansible bootstrap-servers
    ```

7. 执行如下命令，进行 `kubernetes` 的集群安装
    ``` bash
    kubez-ansible deploy
    ```

8. 验证环境
   ```bash
   # kubectl get node
   NAME    STATUS   ROLES    AGE    VERSION
   pixiu   Ready    master   134d   v1.23.6
   ```

9. (可选)启用 kubectl 命令行补全
    ``` bash
    kubez-ansible post-deploy
    ```

