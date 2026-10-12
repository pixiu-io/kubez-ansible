# 安装 containerd

1. [离线安装](offline-containerd.md)
2. [在线安装](online-containerd.md)

```bash
# 验证 containerd 安装完成
nerdctl -n default ps -a
```

# 数据库
```bash
# 快速启动数据库，并初始化 pixiu 数据库（生产环境自行部署或者使用高可用数据库）
nerdctl -n default run -d --restart=always --net host --privileged=true --name mariadb -e MYSQL_ROOT_PASSWORD="Pixiu868686" -e MYSQL_DATABASE="pixiu" ccr.ccs.tencentyun.com/pixiucloud/mysql:5.7
```

# 获取部署驱动镜像
```shell
# 可选，如果没有部署k8s需求，或者可联网可跳过，pixiu 部署时会自行同步 runner
nerdctl -n default pull crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/kubez-ansible:v3.0.4
```

# 启动 pixiu 服务端

## 配置 pixiu
```bash
# 创建配置文件夹
mkdir -p /etc/pixiu/
# 后端配置(host 根据实际情况调整)
vim /etc/pixiu/config.yaml 写入后端如下配置

### 配置文件内容
default:
  # 自动创建指定模型的数据库表结构，不会更新已存在的数据库表
  auto_migrate: true

  # 超级管理初始化用户名和密码；不指定的情况下，默认为 admin/Pixiu123456!
  admin_user: admin
  admin_password: Pixiu123456!

# 数据库地址信息, 根据实际情况配置
mysql:
  host: pixiu # 数据库的ip
  user: root
  password: Pixiu868686
  port: 3306
  name: pixiu
```

## 启动 pixiu
```bash
# 启动 pixiu（/run/containerd 与 /var/lib/containerd 为宿主 containerd 状态路径，必须整目录共享：
nerdctl -n default run -d --restart=always --net host --privileged=true \
  -v /etc/pixiu:/etc/pixiu \
  -v /run/containerd:/run/containerd \
  -v /var/lib/containerd:/var/lib/containerd \
  --name pixiu crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

## 验证
```bash
root@VM-0-11-ubuntu:~# nerdctl ps
CONTAINER ID    IMAGE                                                                                          COMMAND                   CREATED          STATUS    PORTS    NAMES
672a3cbb360d    crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1    "/docker-entrypoint.…"    5 seconds ago    Up                 pixiu
827b5b7ed697    ccr.ccs.tencentyun.com/pixiucloud/mysql:5.7                                                    "docker-entrypoint.s…"    8 minutes ago    Up                 mariadb
```
