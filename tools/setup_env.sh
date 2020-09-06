#!/usr/bin/env bash

curl http://mirrors.aliyun.com/repo/Centos-7.repo -o /etc/yum.repos.d/CentOS-Base.repo
yum install -y epel-release
yum install -y git python-pip ansible

mkdir -p ~/.pip
cat << EOF > ~/.pip/pip.conf
[global]
trusted-host =  mirrors.aliyun.com
index-url = http://mirrors.aliyun.com/pypi/simple/
EOF

if [[ ! -d /tmp/kubez-ansible ]]; then
    git clone https://github.com/yingjuncao/kubez-ansible /tmp/kubez-ansible
    cp -r /tmp/kubez-ansible/etc/kubez/ /etc/
    cp /tmp/kubez-ansible/ansible/inventory/multinode .
fi
pip install /tmp/kubez-ansible/
