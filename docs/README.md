# 前置准备
```bash
准备部署虚拟机节点
```

# 安装 Pixiu
[基于 containerd 安装](containerd-install.md)

[基于 docker 安装](docker-install.md)

## 登陆 pixiu
```
# 使用配置文件中指定的 admin_user / admin_password 登录；
# admin_password 留空时首次启动已自动生成随机密码，见 pixiu 容器启动日志（klog Warning 输出）
浏览器登陆: http://<ip>:<port>
```

## 页面效果
![img.png](images/img.png)