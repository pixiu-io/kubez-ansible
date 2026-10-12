# Pixiu Kubernetes 安装

## 前置

- 可用的 Kubernetes 集群与 `kubectl`
- 可用的 MySQL（需先建库 `pixiu`）
- 能拉取镜像（或已提前导入）：
  - `crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1`

## 1. 准备 MySQL

任选其一。

### 方式 A：已有数据库

```sql
CREATE DATABASE pixiu;
```

### 方式 B：Docker 快速启动

```bash
docker run -d --net host --restart=always --privileged=true \
  --name mariadb \
  -e MYSQL_ROOT_PASSWORD="Pixiu868686" \
  -e MYSQL_DATABASE="pixiu" \
  ccr.ccs.tencentyun.com/pixiucloud/mysql:5.7
```

宿主机访问库地址一般为节点 IP，端口 `3306`。

此时 ConfigMap 中可将 `mysql.host` 设为 `pixiu-mysql`。

## 2. 配置并安装 Pixiu

1. 编辑 `pixiu.yaml` 中 ConfigMap 的 `mysql.host/user/password/port/name`，指向上一步数据库
2. 部署：

```bash
kubectl apply -f deploy/pixiu/pixiu.yaml
```

## 访问

```bash
kubectl -n pixiu-system get svc pixiu
```

浏览器打开：`http://<节点IP>:<NodePort>`
默认账号（与 install.md 一致，可在 ConfigMap 修改）：

- 用户名：`admin`
- 密码：`Pixiu123456!`
