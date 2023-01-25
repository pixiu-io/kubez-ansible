#!/usr/bin/env bash

# NTOE(caoyingjun): Ths script complete the helm installation.

NAMESPACE=$1

function is_helm_installed {
    type helm >/dev/null 2>&1;
    [[ "$?" == "0" ]]
}

function install_helm {
    if ! is_helm_installed; then
        # TODO: Wait for helm_toolbox up

        # install helm
        kubectl cp helm-toolbox-0:usr/bin/helm /usr/bin/helm -n ${NAMESPACE}
        if [ $? -ne 0 ]; then
            echo "failed to copy helm from helm-toolbox" 1>&2
            exit 1
        fi

        chmod +x /usr/bin/helm
    else
        echo "Helm has been installed" 1>&2
    fi
}

install_helm
