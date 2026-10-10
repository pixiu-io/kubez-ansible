# 安装 docker

1. [离线安装](offline-docker.md)
2. [在线安装](online-docker.md)

```bash
# 验证 docker 安装完成
docker ps -a 
```

# 数据库
```bash
# 选择1：直接提供可用数据库，初始化 pixiu 数据库（CREATE DATABASE pixiu;）

# 选择2：快速启动数据库，并初始化 pixiu 数据库（生产环境自行部署或者使用高可用数据库）
# 3306 仅绑定 127.0.0.1，不暴露公网；生产环境必须把 change-me-strong-password 改为强口令
docker run -d -p 127.0.0.1:3306:3306 --restart=always --security-opt no-new-privileges --name mariadb -e MYSQL_ROOT_PASSWORD="change-me-strong-password" -e MYSQL_DATABASE="pixiu" ccr.ccs.tencentyun.com/pixiucloud/mysql:5.7
```

# 获取部署驱动镜像（可选，如果没有部署k8s需求，或者可联网可跳过，pixiu 部署时会自行同步 runner）
```shell
docker pull crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/kubez-ansible:v3.0.4
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

runtime:
  # 宿主容器运行时类型：docker / containerd，默认 containerd；docker 部署须显式写 docker
  cri: docker
  # 宿主运行时 socket 路径，只支持裸路径（如 /run/containerd/containerd.sock），不要写 unix:// 前缀
  # 说明：runtime 配置在服务启动时不校验，配置有误将在首次执行部署任务时报错
  #socket: /run/containerd/containerd.sock

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
# 根据实际需要修改宿主机端口，默认使用宿主机端口，可替换 --net host 为期望端口映射 -p <hostPort>:80
docker run -d --net host --restart=always --security-opt no-new-privileges -v /etc/pixiu:/etc/pixiu -v /var/run/docker.sock:/var/run/docker.sock --name pixiu crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

说明：pixiu 本身无需特权模式（--privileged）；其拉起的部署容器由它经 docker.sock 自行创建。
挂载 docker.sock 后容器内进程等效持有宿主 root 权限，属产品功能依赖的残余风险，请确保仅可信网络可访问 pixiu。

## 登陆 pixiu
```
# 使用配置文件中指定的 admin_user / admin_password 登录；
# admin_password 留空时首次启动已自动生成随机密码，见 pixiu 容器启动日志（klog Warning 输出）
浏览器登陆: http://<ip>:<port>
```
