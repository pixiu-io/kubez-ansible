#!/usr/bin/env bash
#
# This script is intended to be used for clone kubez-ansible to /tmp.

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

function clone_kubez_ansible {
    if [[ ! -d /tmp/kubez-ansible ]]; then
        if is_centos; then
            yum -y install unzip
        elif is_ubuntu; then
            apt-get -y unzip

        curl https://codeload.github.com/caoyingjunz/kubez-ansible/zip/refs/heads/master -o kubez-ansible-master.zip
        if [ $? -ne 0 ]; then
            exit 1
        fi
        unzip kubez-ansible-master.zip && mv kubez-ansible-master /tmp/kubez-ansible && cd /tmp/kubez-ansible && git init
    fi
}

clone_kubez_ansible
