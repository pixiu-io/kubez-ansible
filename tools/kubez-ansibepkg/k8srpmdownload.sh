#!/bin/sh
# 可以使用该脚本下载相关物料包

# 1. nexus安装包
curl -fL -u ptx9sk7vk7ow:003a1d6132741b195f332b815e8f98c39ecbcc1a "https://pixiupkg-generic.pkg.coding.net/pixiu/gopixiu-io/nexus.tar.gz?version=v2" -o nexus.tar.gz

# 2. docker安装包
curl -fL -u ptx9sk7vk7ow:003a1d6132741b195f332b815e8f98c39ecbcc1a "https://pixiupkg-generic.pkg.coding.net/pixiu/gopixiu-io/dockerinstall.tar.gz?version=v2" -o dockerinstall.tar.gz

# 3. ansible安装包
curl -fL -u ptx9sk7vk7ow:003a1d6132741b195f332b815e8f98c39ecbcc1a "https://pixiupkg-generic.pkg.coding.net/pixiu/gopixiu-io/ansibleinstall.tar.gz?version=v2" -o ansibleinstall.tar.gz

# 4. 镜像包
curl -fL -u ptx9sk7vk7ow:003a1d6132741b195f332b815e8f98c39ecbcc1a "https://pixiupkg-generic.pkg.coding.net/pixiu/gopixiu-io/k8soffimage.tar.gz?version=v2" -o k8soffimage.tar.gz
