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
    deb http://mirrors.tuna.tsinghua.edu.cn/ubuntu/ focal main restricted
    deb http://mirrors.tuna.tsinghua.edu.cn/ubuntu/ focal-updates main restricted
    deb http://mirrors.tuna.tsinghua.edu.cn/ubuntu/ focal universe
    deb http://mirrors.tuna.tsinghua.edu.cn/ubuntu/ focal-updates universe
    deb http://mirrors.tuna.tsinghua.edu.cn/ubuntu/ focal multiverse
    deb http://mirrors.tuna.tsinghua.edu.cn/ubuntu/ focal-updates multiverse
    deb http://mirrors.tuna.tsinghua.edu.cn/ubuntu/ focal-backports main restricted universe multiverse
    deb http://mirrors.tuna.tsinghua.edu.cn/ubuntu/ focal-security main restricted
    deb http://mirrors.tuna.tsinghua.edu.cn/ubuntu/ focal-security universe
    deb http://mirrors.tuna.tsinghua.edu.cn/ubuntu/ focal-security multiverse
    deb [arch=amd64] https://download.docker.com/linux/ubuntu xenial stable
    deb http://apt.kubernetes.io/ kubernetes-xenial main
EOF

    sudo apt -y update
    apt install -y ansible python2 git curl
    curl https://bootstrap.pypa.io/get-pip.py --output get-pip.py
    ln /usr/bin/python2 /usr/bin/python
    sudo python2 get-pip.py
}

mkdir -p ~/.pip
cat << EOF > ~/.pip/pip.conf
[global]
trusted-host =  mirrors.aliyun.com
index-url = http://mirrors.aliyun.com/pypi/simple/
EOF

if [[ ! -d /tmp/kubez-ansible ]]; then
    git clone https://github.com/yingjuncao/kubez-ansible /tmp/kubez-ansible
    cp -r /tmp/kubez-ansible/etc/kubez/ /etc/
fi

pip install /tmp/kubez-ansible/

kubez-ansible bootstrap-servers && \
kubez-ansible deploy && \
kubez-ansible post-deploy

kubectl get node
