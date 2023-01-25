#!/usr/bin/env bash
#
# Bootstrap script to install kubernetes env.
#
# This script is intended to be used for install kubernetes env .

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
        # Install the dependence package for centos
        yum -y install epel-release curl unzip
        curl http://mirrors.aliyun.com/repo/Centos-7.repo -o /etc/yum.repos.d/CentOS-Base.repo
        yum -y install git python-pip
    elif is_ubuntu; then
        if [[ "$(systemctl is-enabled ufw)" == "active" ]]; then
            systemctl disable ufw
        fi
        if [[ "$(systemctl is-active ufw)" == "enabled" ]]; then
            systemctl stop ufw
        fi
        # Install the dependence package for ubuntu
        apt install -y curl unzip
        curl -fsSL https://mirrors.aliyun.com/kubernetes/apt/doc/apt-key.gpg | sudo apt-key add
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add
        apt-get install -y software-properties-common
        add-apt-repository "deb [arch=amd64] https://mirrors.aliyun.com/kubernetes/apt/ kubernetes-xenial main"
        add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu/ bionic stable"
        apt-get update
        apt install -y git python-pip
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
    cat > ~/.pip/pip.conf << EOF
[global]
trusted-host = mirrors.aliyun.com
index-url = http://mirrors.aliyun.com/pypi/simple/
EOF
}

function install_ansible {
    if is_centos; then
        yum -y install ansible
    elif is_ubuntu; then
        apt-get -y install ansible
    else
        echo "Unsupported Distro: $DISTRO" 1>&2
        exit 1
    fi
}

function install_kubez_ansible {
    if [[ ! -d /tmp/kubez-ansible ]]; then
        cd /tmp
        curl https://codeload.github.com/caoyingjunz/kubez-ansible/zip/refs/heads/master -o kubez-ansible-master.zip
        if [ $? -ne 0 ]; then
            exit 1
        fi

        unzip kubez-ansible-master.zip
        mv kubez-ansible-master kubez-ansible


    fi
    # prepare the configuration for deploy
    cp -r /tmp/kubez-ansible/etc/kubez/ /etc/
    cp /tmp/kubez-ansible/ansible/inventory/multinode .

    install_ansible

    pip install pbr>=2.0.0
    pip install /tmp/kubez-ansible/
}

# prepare and install kubernetes cluster
prep_work
configure_pip
# cleanup
install_kubez_ansible
