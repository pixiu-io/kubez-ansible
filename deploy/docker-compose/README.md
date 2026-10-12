# 前置准备
```bash
确保 docker 和 docker-compose 已经安装
```

# 获取部署驱动镜像（可选）

如果没有部署k8s需求，或者可联网可跳过，pixiu 部署时会自行拉取镜像

```shell
docker pull ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v2.0.2
docker pull ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v3.0.4
```

# 启动 pixiu

将`docker-compose.yaml`和`config.yaml` 放到 `/etc/pixiu`目录下

```bash
docker-compose up -d
```

验证
```bash
# docker-compose ps
NAME                IMAGE                                                   COMMAND                  SERVICE             CREATED             STATUS              PORTS
mysql               ccr.ccs.tencentyun.com/pixiucloud/mysql:5.7             "docker-entrypoint.s…"   mysql               17 minutes ago      Up 17 minutes       3306/tcp, 33060/tcp
pixiu               crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1   "/docker-entrypoint.…"   pixiu               17 minutes ago      Up 17 minutes       0.0.0.0:8080->80/tcp, :::8080->80/tcp
```

## 登陆 pixiu
```
# 根据配置文件中指定的账密输入；如果未指定默认用户名密码是 admin/Pixiu123456!
浏览器登陆: http://<ip>:8080
```
