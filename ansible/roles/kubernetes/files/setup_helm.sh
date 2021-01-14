#!/usr/bin/env bash

# NTOE(caoyingjun): Ths script complete the helm3 installed for temp,
# but it will be optimise laster

IMAGE=$1

function is_docker_installed {
    type docker >/dev/null 2>&1;
    [[ "$?" == "0" ]]
}

function is_helm_installed {
    type helm >/dev/null 2>&1;
    [[ "$?" == "0" ]]
}

function helm_installed {
    if is_helm_installed; then
        echo "Helm has been installed" 1>&2
    else
        if is_docker_installed; then
            docker run -d --name helm_toolbox $IMAGE && \
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