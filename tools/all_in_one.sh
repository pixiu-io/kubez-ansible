#!/usr/bin/env bash
#
# Bootstrap script to install all-in-one kubernetes cluster.
#
# This script is intended to be used for install all-in-one kubernetes cluster .

function _ensure_lsb_release {
    if type lsb_release >/dev/null 2>&1; then
        return
    fi

    if type apt-get >/dev/null 2>&1; then
        apt-get -y install lsb-release
    elif type yum >/dev/null 2>&1; then
        yum -y install redhat-lsb-core
    fi
}

function _is_distro {
    if [[ -z "$DISTRO" ]]; then
        _ensure_lsb_release
        DISTRO=$(lsb_release -si)
    fi

    [[ "$DISTRO" == "$1" ]]
}

function is_ubuntu {
    _is_distro "Ubuntu"
}

function is_centos {
    _is_distro "CentOS"
}

function prep_work {
    if is_centos; then
        if [[ "$(systemctl is-enabled firewalld)" == "active" ]]; then
            systemctl disable firewalld
        fi
        if [[ "$(systemctl is-active firewalld)" == "enabled" ]]; then
            systemctl stop firewalld
        fi

        # Use aliyun yums
        curl http://mirrors.aliyun.com/repo/Centos-7.repo -o /etc/yum.repos.d/CentOS-Base.repo
        yum -y install epel-release
        yum -y install yum install -y git python-pip ansible
    elif is_ubuntu; then
        if [[ "$(systemctl is-enabled ufw)" == "active" ]]; then
            systemctl disable ufw
        fi
        if [[ "$(systemctl is-active ufw)" == "enabled" ]]; then
            systemctl stop ufw
        fi
        curl -fsSL https://mirrors.aliyun.com/kubernetes/apt/doc/apt-key.gpg | sudo apt-key add
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add
        add-apt-repository "deb [arch=amd64] https://mirrors.aliyun.com/kubernetes/apt/ kubernetes-xenial main"
        add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu/ bionic stable"
        apt-get update
        apt install -y git python-pip ansible
    else
        echo "Unsupported Distro: $DISTRO" 1>&2
        exit 1
    fi
}

function cleanup {
    if is_centos; then
        yum clean all
    elif is_ubuntu; then
        apt-get clean
    else
        echo "Unsupported Distro: $DISTRO" 1>&2
        exit 1
    fi
}

function configure_pip {
    mkdir -p ~/.pip
    cat > ~/.pip/pip.conf >> EOF
[global]
trusted-host = mirrors.aliyun.com
index-url = http://mirrors.aliyun.com/pypi/simple/
EOF
}

function install_kollaz_ansible {
    if [[ ! -d /tmp/kubez-ansible ]]; then
        git clone https://github.com/yingjuncao/kubez-ansible /tmp/kubez-ansible
        cp -r /tmp/kubez-ansible/etc/kubez/ /etc/
    fi

    pip install /tmp/kubez-ansible/
    kubez-ansible bootstrap-servers && kubez-ansible deploy && kubez-ansible post-deploy
}

# prepare and install kubernetes cluster
prep_work
configure_pip
cleanup
install_kollaz_ansible
