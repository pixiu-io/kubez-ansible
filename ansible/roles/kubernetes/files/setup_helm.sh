#!/usr/bin/env bash

# NTOE(caoyingjun): Ths script complete the helm3 installed for temp,
# but it will be optimise laster

IMAGE=$1

function _helm_installed {
    if type helm >/dev/null 2>&1; then
        return
    fi

    if type apt-get >/dev/null 2>&1; then
        apt-get -y install lsb-release
    elif type yum >/dev/null 2>&1; then
        yum -y install redhat-lsb-core
    fi
}


docker run -d --name helm_toolbox $IMAGE
docker cp helm_toolbox:/usr/bin/helm /usr/bin/helm
chmod +x /usr/bin/helm
docker rm helm_toolbox -f
