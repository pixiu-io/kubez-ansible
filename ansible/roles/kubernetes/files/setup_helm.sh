#!/usr/bin/env bash

# NTOE(caoyingjun): Ths script complete the helm3 installed for temp,
# but it will be optimise laster

function _is_installed {
    type $1 >/dev/null 2>&1;
    [[ "$?" == "0" ]]
}

function is_docker_installed {
    _is_installed "docker"
}

function is_helm_installed {
    _is_installed "helm"
}

function helm_installed {
    if is_helm_installed; then
        echo "Helm has been installed" 1>&2
        exit 0
    else
        if is_docker_installed; then
            docker run -d --name helm_toolbox jacky06/helm-toolbox:v3.0.3 && \
            docker cp helm_toolbox:/usr/bin/helm /usr/bin/helm && \
            chmod +x /usr/bin/helm && \
            docker rm helm_toolbox -f
        else
           echo "Docker is not installed, but necessary" 1>&2
           exit 1
        fi
    fi
}

is_docker_installed
helm_installed
