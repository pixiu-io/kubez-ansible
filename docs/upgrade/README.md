# 手动升级

根据当初的安装方式选择对应步骤。升级只替换 pixiu 容器，mysql 和 `/etc/pixiu` 配置保持不动。

将下文中的镜像标签换成目标版本（当前示例为 `v2.0.2-beta.1`）。

## 基于 docker-compose 安装

进入部署目录，先改 `docker-compose.yaml` 里 pixiu 的 image 标签，再拉镜像并重建容器：

```bash
cd /etc/pixiu

# 修改 pixiu 镜像版本
vim docker-compose.yaml
```

拉取镜像

```bash
# pixiu
docker pull crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

重建 pixiu

```bash
docker-compose up -d pixiu
```

验证

```bash
docker-compose ps
```

## 基于手动安装（containerd）

containerd 形态是 `nerdctl` 命令式启动的，升级步骤与 docker 版一致：先拉新镜像，再删掉旧容器后按原参数重新启动。

拉取镜像

```bash
# pixiu
nerdctl -n default pull crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

替换容器

```bash
nerdctl -n default stop pixiu
nerdctl -n default rm pixiu

# 参数与 containerd-install.md 保持一致，仅替换镜像版本（/run/containerd 与 /var/lib/containerd 须整目录挂载）
nerdctl -n default run -d --net host --restart=always --privileged=true \
  -v /etc/pixiu:/etc/pixiu \
  -v /run/containerd:/run/containerd \
  -v /var/lib/containerd:/var/lib/containerd \
  --name pixiu \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

## 基于手动安装（docker）

手动安装是 `docker run` 启动的，需要先拉新镜像，再删掉旧容器后按原参数重新启动：

拉取镜像

```bash
# pixiu
docker pull crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

替换容器

```bash
docker stop pixiu
docker rm pixiu

# 参数与 install.md 保持一致，仅替换镜像版本
docker run -d --net host --restart=always --privileged=true \
  -v /etc/pixiu:/etc/pixiu \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --name pixiu \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```
