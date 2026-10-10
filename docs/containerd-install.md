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
# 3306 仅绑定 127.0.0.1，不暴露公网；生产环境必须把 change-me-strong-password 改为强口令
nerdctl -n default run -d --restart=always -p 127.0.0.1:3306:3306 --security-opt no-new-privileges --name mariadb -e MYSQL_ROOT_PASSWORD="change-me-strong-password" -e MYSQL_DATABASE="pixiu" ccr.ccs.tencentyun.com/pixiucloud/mysql:5.7
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

  # jwt 签名的 key（必填）：release 模式下为空或仍为示例值将拒绝启动，
  # 请设置强随机值，生成方式: openssl rand -hex 32
  jwt_key: ""

  # 超级管理初始化用户名；密码留空时首次启动自动生成随机强密码并输出到启动日志（请及时修改）
  # 生产环境必须设置强口令，禁止使用任何示例/默认口令
  admin_user: admin
  admin_password: ""

# 数据库地址信息, 根据实际情况配置
# 生产环境必须改为强口令，数据库不得暴露公网
mysql:
  host: 127.0.0.1 # 数据库的ip
  user: root
  password: change-me-strong-password
  port: 3306
  name: pixiu
```

## 启动 pixiu
```bash
# 启动 pixiu（/run/containerd 与 /var/lib/containerd 为宿主 containerd 状态路径，必须整目录共享：
# 说明：pixiu 本身无需特权模式（--privileged）；其拉起的部署容器由它经 containerd 状态路径自行创建
nerdctl -n default run -d --restart=always --net host --security-opt no-new-privileges \
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
