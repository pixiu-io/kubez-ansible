#!/usr/bin/env bash
#
# Bootstrap script to install kubernetes env.
#
# This script is intended to be used for install kubernetes env.

REPO=gopixiu-io
# 选择需要安装的分支，默认 master 分支
BRANCH=master

TARGET=kubez-ansible-${BRANCH//\//-}

function _ensure_lsb_release {
    if type lsb_release >/dev/null 2>&1; then
        return
    fi

    if type apt-get >/dev/null 2>&1; then
        apt-get -y install lsb-release
    elif type yum >/dev/null 2>&1; then
        yum -y install redhat-lsb-core
    fi

    if type dnf >/dev/null 2>&1; then
        dnf -y install redhat-lsb-core
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

function is_debian {
    _is_distro "Debian"
}

function is_centos {
    _is_distro "CentOS"
}

function is_rocky {
    _is_distro "Rocky"
}

function ensure_python3_installed {
    if type python3 >/dev/null 2>&1; then
        return
    else
        echo "python3 not be found" 1>&2
        echo "CentOS 7, Debian 10 and Ubuntu 18.04 with python2 should refer to https://github.com/gopixiu-io/kubez-ansible/tree/stable/tiger" 1>&2
        exit 1
    fi
}

function prep_work {
    if is_rocky; then
        if [[ "$(systemctl is-enabled firewalld)" == "active" ]]; then
            systemctl disable firewalld
        fi
        if [[ "$(systemctl is-active firewalld)" == "enabled" ]]; then
            systemctl stop firewalld
        fi
        configure_rocky_souces
        dnf -y install epel-release
        dnf -y install git python3-pip unzip

    elif is_ubuntu || is_debian; then
        if [[ "$(systemctl is-enabled ufw)" == "active" ]]; then
            systemctl disable ufw
        fi
        if [[ "$(systemctl is-active ufw)" == "enabled" ]]; then
            systemctl stop ufw
        fi

        if is_debian; then
            configure_debian_sources
        else
            configure_ubuntu_sources
        fi
        apt-get update
        apt install -y git python3-pip unzip
    else
        echo "当前版本不支持: $DISTRO" 1>&2
        echo "CentOS 7, Debian 10 & Ubuntu 18.04 等操作系统，请参考 https://github.com/gopixiu-io/kubez-ansible/tree/stable/tiger" 1>&2

        echo "Unsupported Distro: $DISTRO" 1>&2
        echo "CentOS 7, Debian 10 and Ubuntu 18.04 with python2 should refer to https://github.com/gopixiu-io/kubez-ansible/tree/stable/tiger" 1>&2
        exit 1
    fi
}

function cleanup {
    if is_centos; then
        yum clean all
    elif is_ubuntu || is_debian; then
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

function configure_centos_sources {
    if [ ! -f "/etc/yum.repos.d/CentOS-Base.repo.backup" ];then
         mv /etc/yum.repos.d/CentOS-Base.repo /etc/yum.repos.d/CentOS-Base.repo.backup
    fi
    # CentOS 7
    curl http://mirrors.aliyun.com/repo/Centos-7.repo -o /etc/yum.repos.d/CentOS-Base.repo
}

function configure_rocky_souces {
    sed -e 's|^mirrorlist=|#mirrorlist=|g' \
    -e 's|^#baseurl=http://dl.rockylinux.org/$contentdir|baseurl=https://mirrors.aliyun.com/rockylinux|g' \
    -i.bak \
    /etc/yum.repos.d/Rocky-*.repo
}

function configure_debian_sources {
    if [ ! -f "/etc/apt/sources.list.backup" ];then
         mv /etc/apt/sources.list /etc/apt/sources.list.backup
    fi

    UBUNTU_CODENAME=$(cat /etc/os-release |egrep "^VERSION_CODENAME=\"*(\w+)\"*" |awk -F= '{print $2}' |tr -d '\"')
    # debian 11.x+
    cat > /etc/apt/sources.list << EOF
deb https://mirrors.aliyun.com/debian/ ${UBUNTU_CODENAME} main non-free contrib
deb-src https://mirrors.aliyun.com/debian/ ${UBUNTU_CODENAME} main non-free contrib
deb https://mirrors.aliyun.com/debian-security/ ${UBUNTU_CODENAME}-security main
deb-src https://mirrors.aliyun.com/debian-security/ ${UBUNTU_CODENAME}-security main
deb https://mirrors.aliyun.com/debian/ ${UBUNTU_CODENAME}-updates main non-free contrib
deb-src https://mirrors.aliyun.com/debian/ ${UBUNTU_CODENAME}-updates main non-free contrib
deb https://mirrors.aliyun.com/debian/ ${UBUNTU_CODENAME}-backports main non-free contrib
deb-src https://mirrors.aliyun.com/debian/ ${UBUNTU_CODENAME}-backports main non-free contrib
EOF
}

function configure_ubuntu_sources() {
    if [ ! -f "/etc/apt/sources.list.backup" ];then
        mv /etc/apt/sources.list /etc/apt/sources.list.backup
    fi

    UBUNTU_CODENAME=$(cat /etc/os-release |egrep "^VERSION_CODENAME=\"*(\w+)\"*" |awk -F= '{print $2}' |tr -d '\"')
    cat > /etc/apt/sources.list << EOF
deb https://mirrors.aliyun.com/ubuntu/ ${UBUNTU_CODENAME} main restricted universe multiverse
deb-src https://mirrors.aliyun.com/ubuntu/ ${UBUNTU_CODENAME} main restricted universe multiverse

deb https://mirrors.aliyun.com/ubuntu/ ${UBUNTU_CODENAME}-security main restricted universe multiverse
deb-src https://mirrors.aliyun.com/ubuntu/ ${UBUNTU_CODENAME}-security main restricted universe multiverse

deb https://mirrors.aliyun.com/ubuntu/ ${UBUNTU_CODENAME}-updates main restricted universe multiverse
deb-src https://mirrors.aliyun.com/ubuntu/ ${UBUNTU_CODENAME}-updates main restricted universe multiverse

deb https://mirrors.aliyun.com/ubuntu/ ${UBUNTU_CODENAME}-backports main restricted universe multiverse
deb-src https://mirrors.aliyun.com/ubuntu/ ${UBUNTU_CODENAME}-backports main restricted universe multiverse
EOF
}

function install_ansible {
    if is_centos; then
        yum -y install ansible
    elif is_ubuntu || is_debian; then
        apt-get -y install ansible
    elif is_rocky; then
        dnf -y install ansible
    else
        echo "Unsupported Distro: $DISTRO" 1>&2
        exit 1
    fi
}

function download_kubez_ansible {
    curl https://codeload.github.com/${REPO}/kubez-ansible/zip/refs/heads/${BRANCH} -o ${TARGET}.zip
    if [ $? -ne 0 ]; then
        exit 1
    fi

    unzip -q ${TARGET}.zip && mv ${TARGET} /tmp/kubez-ansible && git init /tmp/kubez-ansible
}

function install_kubez_ansible {
    if [[ ! -d /tmp/kubez-ansible ]]; then
        download_kubez_ansible
    fi
    # prepare the configuration for deploy
    cp -r /tmp/kubez-ansible/etc/kubez/ /etc/
    cp /tmp/kubez-ansible/ansible/inventory/multinode .

    install_ansible

    if is_rocky; then
        # TODO: ansible will search the kubez_ansible plugin from python3.9
        python_version=$(python3 -c "import sys;print(sys.version[2])")
        cp -r /usr/local/lib/python3.${python_version}/site-packages/kubez_ansible /usr/lib/python3.9/site-packages/
    fi

    pip3 install -r /tmp/kubez-ansible/requirements.txt
    pip3 install /tmp/kubez-ansible/
}

ensure_python3_installed

# prepare and install kubernetes cluster
prep_work
configure_pip
# cleanup
install_kubez_ansible
