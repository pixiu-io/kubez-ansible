#!/usr/bin/env bash

# NTOE(caoyingjun): Ths script complete the helm3 installation for centos.

IMAGE=$1

function is_docker_installed {
    type docker >/dev/null 2>&1;
    [[ "$?" == "0" ]]
}

function is_helm_installed {
    type helm >/dev/null 2>&1;
    [[ "$?" == "0" ]]
}

function toolbox_image_exists {
    REPO=$(echo ${IMAGE} | awk -F ':' '{print $1}')
    TAG=$(echo ${IMAGE} | awk -F ':' '{print $2}')
    docker images | grep ${REPO} | grep ${TAG}
    [[ "$?" == "0" ]]
}

function helm_installed {
    if ! is_helm_installed; then
        if is_docker_installed; then
            # Pull the toolbox image when not exists
            if ! toolbox_image_exists; then
                docker pull ${IMAGE}
            fi

            # Install the command by copy from container
            docker run -d --name helm_toolbox ${IMAGE} && \
            docker cp helm_toolbox:/usr/bin/helm /usr/bin/helm && \
            chmod +x /usr/bin/helm && \
            docker rm helm_toolbox -f
            echo "Install helm done" 1>&2
        else
            # For now, the docker is necessary when helm installed
            echo "Docker is not installed, but necessary" 1>&2
            exit 1
        fi
    else
        echo "Helm has been installed" 1>&2
    fi
}

helm_installed