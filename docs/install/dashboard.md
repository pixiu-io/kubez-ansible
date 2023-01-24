# 部署 dashboard

### 证书制作
``` bash
# 生成证书请求的key
openssl genrsa -out dashboard.key 2048

# 生成证书请求
openssl req -days 3650 -new -key dashboard.key -out dashboard.csr  -subj /C=CN/ST=HZ/L=BJ/CN=*.kirin.com.cn

# 生成自签证书（证书文件 dashboard.crt 和密钥 dashboard.key ）
openssl x509 -req -in dashboard.csr -signkey dashboard.key -out dashboard.crt

# 查看证书信息
openssl x509 -in dashboard.crt -text -noout
```

# 创建 kubernetes-dashboard-certs
``` bash
# 假设证书在 $HOME/certs 目录中
kubectl create secret generic kubernetes-dashboard-certs --from-file=$HOME/certs -n kubernetes-dashboard
```
