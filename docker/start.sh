#!/bin/sh
set -o errexit
set -o xtrace

if [[ ! -d "/etc/kubez" ]]; then
    mkdir -p /etc/kubez
fi
cp /configs/globals.yml /etc/kubez/globals.yml
cp /configs/multinode /etc/kubez/multinode

kubez-ansible -i /etc/kubez/multinode deploy
