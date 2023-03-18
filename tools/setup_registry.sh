#!/usr/bin/env bash
#
# Bootstrap script to install nexus.
#
# This script is intended to be used for install nexus server for offline env.

function setup_nexus() {
    if [ ! -d "/data" ]; then
        mkdir /data
    fi

    if [ ! -d "/data/nexus" ]; then
        if [ ! -e "./nexus.tar.gz" ]; then
            echo "当前目录中未发现 nexus.tar.gz，无法进行 nexus 的安装" 1>&2
            exit 1
        fi
        tar -zxvf ./nexus.tar.gz -C /data
    fi

     # 启动 nexus.sh
     cd /data/nexus && sh nexus.sh start

     yum clean all
     echo "nexus 安装成功"
}

setup_nexus
