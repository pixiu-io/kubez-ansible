# 离线部署

### 下载离线包，并传到离线部署节点
- builder 二进制和 pixiu 镜像包 获取 [Pixiu基础包](https://github.com/offline-hub/repo/releases/tag/download)
- k8s 镜像包获取 [镜像包](https://github.com/offline-hub/repo/releases/tag/images)
- 安装包获取 [v1.31.6](https://github.com/offline-hub/repo/releases/tag/v1.31.6)

### 离线包部署
以系统 v1.31.6为例，将下载的镜像上传到服务器目录
#### 启动离线仓库
```
# 本例中 builder 是在 /home 目录下
chmod +x builder
# 使用 systemctl 管理
sudo vi /etc/systemd/system/pixiu-builder.service

# 配置如下
[Unit]
Description=Pixiu builder
After=network.target

[Service]
Type=simple
WorkingDirectory=/home
ExecStart=/home/builder serve --dir /home/data
Restart=always
RestartSec=5
Environment="PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

[Install]
WantedBy=multi-user.target

# 配置完加载，并设置开机启动
sudo systemctl daemon-reload
sudo systemctl enable pixiu-builder
sudo systemctl start pixiu-builder
```

### 查看日志
```bash
root@localhost:~# journalctl -u pixiu_builder -f
加载离线产物到 ./serve-data ...
  已加载 0 个 bundle（跳过 0 个）
  未发现 .rpm/.deb，跳过软件源
软件源已启动: http://10.206.32.17:8080 （rpm=0 deb=0）
导入镜像到 registry 10.206.32.17:5000 ...
  已导入 0 个镜像

========== builder serve 就绪 ==========
Registry:  10.206.32.17:5000
  示例:
    docker pull 10.206.32.17:5000/<name>:<tag>
  Docker insecure-registries 需包含: "10.206.32.17:5000"
```

### 加载镜像，安装运行环境
- [Ubuntu24.04](Ubuntu.md)
- [KylinV10](KylinV10.md)

### 安装 pixiu

#### 安装 mysql
```bash
# 生产环境必须把 change-me-strong-password 改为强口令，数据库不得暴露公网
docker run -d --net host --restart=always --security-opt no-new-privileges --name mariadb -e MYSQL_ROOT_PASSWORD="change-me-strong-password" -e MYSQL_DATABASE="pixiu" 10.206.32.8:5000/pixiu/mysql:5.7
```

#### 配置 pixiu
##### 创建配置文件夹 （文件夹路径不可修改）
mkdir -p /etc/pixiu/
vim /etc/pixiu/config.yaml 写入后端如下配置

##### 后端配置(host 根据实际情况调整)
```bash
default:
  auto_migrate: true

  # jwt 签名的 key（必填）：release 模式下为空或仍为示例值将拒绝启动，
  # 请设置强随机值，生成方式: openssl rand -hex 32
  jwt_key: ""

  # 超级管理初始化用户名；密码留空时首次启动自动生成随机强密码并输出到启动日志（请及时修改）
  # 生产环境必须设置强口令，禁止使用任何示例/默认口令
  admin_user: admin
  admin_password: ""

runtime:
  # 宿主容器运行时类型：docker / containerd，默认 containerd；docker 部署须显式写 docker
  cri: docker
  # 宿主运行时 socket 路径，只支持裸路径（如 /run/containerd/containerd.sock），不要写 unix:// 前缀
  #socket: /run/containerd/containerd.sock

# 数据库地址信息, 根据实际情况配置
# 生产环境必须改为强口令，数据库不得暴露公网
mysql:
  host: 10.206.32.8
  user: root
  password: change-me-strong-password
  port: 3306
  name: pixiu
```

#### 安装 pixiu-server
```bash
docker run -d --net host --restart=always --security-opt no-new-privileges -v /etc/pixiu:/etc/pixiu -v /var/run/docker.sock:/var/run/docker.sock --name pixiu 10.206.32.8:5000/pixiu/pixiu:v2.0.2-beta.1
```
![img_4.png](img_4.png)

#### containerd 环境差异（宿主使用 containerd 而非 docker 时）

前置：宿主需已安装并运行 `containerd`，且已安装 `nerdctl`（v2.x，解压官方发布包后放到 `/usr/local/bin`，`nerdctl version` 自检），容器操作需 root。命令与 docker 版逐条对应，差异集中在**镜像导入的命名空间**、**socket 挂载**与**配置中的 `runtime.cri`** 三点（见下文第 3 步）。

1）导出/导入镜像：containerd 的镜像存储按命名空间隔离，导出与导入都要指定同一命名空间。

在**联网机器**导出归档（镜像 tag 换成实际版本；示例与本文命令中的镜像引用一致）：

```bash
sudo nerdctl -n default save -o pixiu-image.tar \
  10.206.32.8:5000/pixiu/pixiu:v2.0.2-beta.1
# 部署 runner 镜像同理（如 kubez-ansible），按需导出
sudo nerdctl -n default save -o kubez-ansible.tar ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v3.0.4
```

（若所用 nerdctl 版本不支持 `-o`，改用标准输出重定向：`sudo nerdctl -n default save <镜像> > pixiu-image.tar`。）

在**离线宿主**导入归档（必须导入到 pixiu 使用的命名空间，默认 `default`；`k8s.io` 里的镜像不会被 pixiu 直接复用）：

```bash
sudo nerdctl -n default load -i pixiu-image.tar       # nerdctl load 支持 Docker v1.2 与 OCI v1.0 两种归档
sudo nerdctl -n default load -i kubez-ansible.tar
sudo nerdctl -n default images                        # 确认已导入
```

- 给 kubelet 用的 k8s 集群运行时镜像走另一条路：`sudo ctr -n k8s.io images import`（如 `zcat images-export/images/*.tar.gz | sudo ctr -n k8s.io images import -`）。它与 pixiu 容器所用的 `default` 命名空间不是一回事，不要混用，也不要指望 pixiu 复用 k8s.io 里的镜像。
- 私有仓库为 HTTP/自签证书时，nerdctl 需加全局 `--insecure-registry`（如 `sudo nerdctl --insecure-registry pull 10.206.32.8:5000/pixiu/pixiu:v2.0.2-beta.1`）。

2）启动：把 docker 的 socket 挂载换成 containerd 状态路径——`-v /run/containerd:/run/containerd`（整目录共享，含 socket）与 `-v /var/lib/containerd:/var/lib/containerd`（pixiu 作为 containerd 本地客户端会在容器内创建 fifo、执行镜像解包挂载，两棵路径须与宿主 daemon/shim 一致）；`--restart=always` 等参数不变；runner 容器日志固定写到 `/etc/pixiu/runner-logs`（随 `/etc/pixiu` 卷持久化，无需额外挂载）。

```bash
# 数据库（生产环境必须把 change-me-strong-password 改为强口令，数据库不得暴露公网）
sudo nerdctl -n default run -d --restart=always --net host --security-opt no-new-privileges --name mariadb -e MYSQL_ROOT_PASSWORD="change-me-strong-password" -e MYSQL_DATABASE="pixiu" 10.206.32.8:5000/pixiu/mysql:5.7

# pixiu（无需特权模式 --privileged；其拉起的部署容器由它经 containerd 状态路径自行创建）
sudo nerdctl -n default run -d --restart=always --net host --security-opt no-new-privileges \
  -v /etc/pixiu:/etc/pixiu \
  -v /run/containerd:/run/containerd \
  -v /var/lib/containerd:/var/lib/containerd \
  --name pixiu 10.206.32.8:5000/pixiu/pixiu:v2.0.2-beta.1
```

3）配置：`/etc/pixiu/config.yaml` 中把运行时指向 containerd，pixiu 才会用 containerd 拉起部署 runner 容器（默认 containerd，不配置即使用 containerd）。

```yaml
runtime:
  cri: containerd
  #socket: /run/containerd/containerd.sock   # 与上方 run 命令中挂载的 socket 路径一致；留空即默认此路径
```

4）验证与卸载：

```bash
sudo nerdctl -n default ps -a
sudo nerdctl -n default logs -f pixiu
sudo nerdctl -n default rm -f pixiu mariadb
```

#### 页面验证
![img_5.png](img_5.png)

### 集群部署

#### 创建部署
指定 Kubernetes 镜像仓库 和 自定义源为私有地址

#### ubuntu配置参考如下：

![img_8.png](img_8.png)

#### 麒麟V10配置参考如下：
![img_7.png](img_7.png)
#### 调整 runner 为私有镜像
![img_9.png](img_9.png)

#### 完成部署
2分钟完成部署
```bash
root@VM-32-8-ubuntu:~# kubectl  get pod -A -o wide
NAMESPACE      NAME                                        READY   STATUS              RESTARTS   AGE     IP               NODE        NOMINATED NODE   READINESS GATES
kube-system    calico-kube-controllers-5d6c89b768-wkhxs    1/1     Running             0          2m53s   172.30.222.194   test-node   <none>           <none>
kube-system    calico-node-zc59j                           1/1     Running             0          2m53s   10.206.32.8      test-node   <none>           <none>
kube-system    calico-typha-c879574bd-4crm6                1/1     Running             0          2m53s   10.206.32.8      test-node   <none>           <none>
kube-system    coredns-798dfbc648-2smkr                    1/1     Running             0          3m14s   172.30.222.198   test-node   <none>           <none>
kube-system    coredns-798dfbc648-9fsp6                    1/1     Running             0          3m14s   172.30.222.196   test-node   <none>           <none>
kube-system    etcd-test-node                              1/1     Running             0          3m21s   10.206.32.8      test-node   <none>           <none>
kube-system    ingress-nginx-admission-create-krmzv        0/1     ImagePullBackOff    0          2m49s   172.30.222.199   test-node   <none>           <none>
kube-system    ingress-nginx-admission-patch-pq6pz         0/1     ImagePullBackOff    0          2m49s   172.30.222.197   test-node   <none>           <none>
kube-system    ingress-nginx-controller-857d64b88c-9njsc   0/1     ContainerCreating   0          2m49s   <none>           test-node   <none>           <none>
kube-system    kube-apiserver-test-node                    1/1     Running             0          3m21s   10.206.32.8      test-node   <none>           <none>
kube-system    kube-controller-manager-test-node           1/1     Running             0          3m21s   10.206.32.8      test-node   <none>           <none>
kube-system    kube-proxy-krv7d                            1/1     Running             0          3m14s   10.206.32.8      test-node   <none>           <none>
kube-system    kube-scheduler-test-node                    1/1     Running             0          3m21s   10.206.32.8      test-node   <none>           <none>
kube-system    metrics-server-5667f666f7-w2dl8             1/1     Running             0          2m52s   172.30.222.193   test-node   <none>           <none>```
```
