#!/usr/bin/env bash

linux_os(){
    a=`uname -a`
    if [[ $a =~ Ubuntu ]];then
            ubuntu_shell ;
    else    centos_shell ;
    fi
    }

centos_shell(){
    curl http://mirrors.aliyun.com/repo/Centos-7.repo -o /etc/yum.repos.d/CentOS-Base.repo
    yum install -y epel-release
    yum install -y git python-pip ansible
}

ubuntu_shell(){
    systemctl disable systemd-resolved
    cp /etc/apt/sources.list /etc/apt/sources.list.bak
    echo "nameserver 114.114.114.114" >>/etc/resolv.conf

    cat << EOF > /etc/apt/sources.list
    deb http://mirrors.aliyun.com/ubuntu/ bionic main restricted universe multiverse
    deb http://mirrors.aliyun.com/ubuntu/ bionic-security main restricted universe multiverse
    deb http://mirrors.aliyun.com/ubuntu/ bionic-updates main restricted universe multiverse
    deb http://mirrors.aliyun.com/ubuntu/ bionic-proposed main restricted universe multiverse
    deb http://mirrors.aliyun.com/ubuntu/ bionic-backports main restricted universe multiverse
    deb-src http://mirrors.aliyun.com/ubuntu/ bionic main restricted universe multiverse
    deb-src http://mirrors.aliyun.com/ubuntu/ bionic-security main restricted universe multiverse
    deb-src http://mirrors.aliyun.com/ubuntu/ bionic-updates main restricted universe multiverse
    deb-src http://mirrors.aliyun.com/ubuntu/ bionic-proposed main restricted universe multiverse
    deb-src http://mirrors.aliyun.com/ubuntu/ bionic-backports main restricted universe multiverse
EOF
    sudo apt -y update
    apt install -y ansible git curl python-pip
}

mkdir -p ~/.pip
cat << EOF > ~/.pip/pip.conf
[global]
trusted-host =  mirrors.aliyun.com
index-url = http://mirrors.aliyun.com/pypi/simple/
EOF

linux_os
if [[ ! -d /tmp/kubez-ansible ]]; then
    git clone https://github.com/yingjuncao/kubez-ansible /tmp/kubez-ansible
    cp -r /tmp/kubez-ansible/etc/kubez/ /etc/
fi

pip install /tmp/kubez-ansible/

kubez-ansible bootstrap-servers && \
kubez-ansible deploy && \
kubez-ansible post-deploy

kubectl get node
