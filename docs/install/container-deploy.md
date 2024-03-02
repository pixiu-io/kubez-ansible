# 容器部署

### 依赖条件
- [Docker安装]()

#### 配置
- 创建配置文件夹
```shell
mkdir -p /etc/kubez
```
- 全局配置
```shell
vim /etc/kubez/globals.yml
```
- multinode 分组
```shell
# vim /etc/kubez/multinode
# 分组中加上用户名和密码
[docker-master]
pixiu01 ansible_ssh_user=root ansible_ssh_port=22 ansible_ssh_pass=123456 ansible_become_password=123456 ansible_become=true ansible_become_user=root
```

- 主机名解析
```shell
# cat /etc/kubez/hosts
127.0.0.1 localhost
::1         localhost localhost.localdomain localhost6 localhost6.localdomain6
# BEGIN ANSIBLE GENERATED HOSTS
192.168.16.200 pixiu01
192.168.16.201 pixiu02
# END ANSIBLE GENERATED HOSTS
```

### 安装
```shell
docker run -d --name kubez-ansible-bootstrap-servers -e COMMAND=bootstrap-servers  -v /etc/kubez:/configs jacky06/kubez-ansible:v3.0.1
docker run -d --name kubez-ansible-deploy -e COMMAND=deploy -v /etc/kubez:/configs jacky06/kubez-ansible:v3.0.1
```

### 验证环境
   ```bash
   [root@kube01 ~]# kubectl get node
   NAME     STATUS   ROLES                  AGE     VERSION
   kube01   Ready    control-plane,master   21h     v1.23.6
   kube02   Ready    <none>                 21h     v1.23.6
   kube03   Ready    <none>                 3h48m   v1.23.6
   ```
