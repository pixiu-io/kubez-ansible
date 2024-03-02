#!/bin/bash
set -o errexit
set -o xtrace

if [[ ! -d "/etc/kubez" ]]; then
    mkdir -p /etc/kubez
fi

cp /configs/globals.yml /etc/kubez/globals.yml
cp /configs/multinode /etc/kubez/multinode
cp /configs/hosts /etc/hosts

kubez-ansible -i /etc/kubez/multinode ${COMMAND}
