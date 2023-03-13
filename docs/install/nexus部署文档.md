# nexus 部署

## 1. 配置hosts
```shell
cat >> /etc/hosts <<\EOF
192.168.153.100    nexus.pixiu.com
EOF
```
192.168.153.100改为你本机ip



## 2. 下载物料包
```shell
curl -fL -u ptx9sk7vk7ow:003a1d6132741b195f332b815e8f98c39ecbcc1a "https://pixiupkg-generic.pkg.coding.net/pixiu/gopixiu-io/nexus.tar.gz?version=v2" -o nexus.tar.gz
```

## 3. 启动服务
```shell
cd nexus && sh nexus.sh start 
```

## 4. 停止服务
```shell
sh nexus.sh stop
```

## 5. 访问服务

```shell
http://nexus.pixiu.com:50000   访问nexus服务地址                         
http://nexus.pixiu.com/repository/yuminstall/  yum仓库地址
http://nexus.pixiu.com:58001     harbor仓库地址
```
-  所有访问 用户名:    admin   密码:   admin@AdMin123

## 6. 查看/etc/yum.repos.d/nexus.repo

```shell
name=Nexus Yum Repository
baseurl=http://nexus.pixiu.com:50000/repository/yuminstall/
enabled=1
gpgcheck=0
```
- baseurl 改为自己的服务器

```shell
yum clean all
```
