# nexus 部署文档

## 1. 基础环境准备

```shell
# nexus物料包
curl -fL -u ptveurhii3a9:e5bcf1b8a42f022a671384f3ab6f405403450110 "https://shujiangle-generic.pkg.coding.net/sjldevops/software/nexus.tar.gz?version=latest" -o nexus.tar.gz
```

## 2. 启动服务
```shell
# 1. 解压物料包
tar -zxvf nexus.tar.gz && cd nexus

# 2. 启动服务
sh nexus.sh start

# 3. 检查服务
sh check_nexus.sh
```

## 3. 停止服务
```shell
sh nexus.sh stop
```

## 4.  访问服务

> http://ip:50000          用户名:    admin   密码:   admin@AdMin123

## 5. 镜像服务访问

### 5.1 配置insecure-registries
```shell
cat >> /etc/docker/daemon.json <\EOF 
{
  "insecure-registries":["192.168.153.101","192.168.153.120:5000","192.168.153.55:58001"],
}
EOF
```

### 5.2 登录服务

```shell
docker login  ip:58001     用户名:    admin   密码:   admin@AdMin123
```
