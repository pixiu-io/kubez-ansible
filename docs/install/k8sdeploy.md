# k8sdeploy
## 1. k8s物料包下载
```shell
# k8s物料包
curl -fL -u ptveurhii3a9:e5bcf1b8a42f022a671384f3ab6f405403450110 "https://shujiangle-generic.pkg.coding.net/sjldevops/software/k8sdeploy.tar.gz?version=latest" -o k8sdeploy.tar.gz
```

## 2. nexus安装
```shell
tar k8sdeploy.tar.gz
cd k8sdeploy && tar -zxvf nexus.tar.gz
cd nexus && sh nexus.sh start
```

## 3. install kubelet-1.23.6 kubeadm-1.23.6 kubectl-1.23.6  and install  helm
```shell
# 重新进入k8sdeploy目录
cd .. && sh k8spkg.sh
```