#!/usr/bin/env bash

# NTOE(caoyingjun): Ths script complete the helm installation.

NAMESPACE=$1

function is_helm_installed {
    type helm >/dev/null 2>&1;
    [[ "$?" == "0" ]]
}

function is_helm_toolbox_created {
    kubectl get pod -n ${NAMESPACE} | grep helm-toolbox-0
    [[ "$?" == "0" ]]
}

function install_helm {
    if ! is_helm_installed; then
        # Wait for helm_toolbox up
        retries=0
        while true; do
            if [ $retries -ge 600 ]; then
                echo "failed to wait helm-toolbox up" 1>&2
                exit 1
            fi

            # wait for helm-toolbox created
            if ! is_helm_toolbox_created; then
                sleep 1
                retries=$(($retries +1))
                continue
            fi

            # wait for helm-toolbox up
            ready=$(kubectl get pod helm-toolbox-0 -n ${NAMESPACE} | grep helm-toolbox-0 | awk '{print $2}')
            if [[ "$ready" == "1/1" ]]; then
                break
            fi

            sleep 1
            retries=$(($retries +1))
        done

        # install helm
        kubectl cp helm-toolbox-0:usr/bin/helm /usr/bin/helm -n ${NAMESPACE} --retries=5
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
