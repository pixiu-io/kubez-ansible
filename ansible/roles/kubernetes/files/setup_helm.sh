#!/usr/bin/env bash

# NTOE(caoyingjun): Ths script complete the helm3 installed for temp,
# but it will be optimise laster

function _is_installed {
    type $1 >/dev/null 2>&1;
    [[ "$?" == "$1" ]]
}

function is_docker_installed {
    _is_installed "docker"
}

function is_helm_installed {
    _is_installed "helm"
}

function helm_installed {
    if !is_helm_installed {
        docker run -d --name helm_toolbox $IMAGE
        docker cp helm_toolbox:/usr/bin/helm /usr/bin/helm
        chmod +x /usr/bin/helm
        docker rm helm_toolbox -f
    }
}

is_docker_installed
helm_installed
