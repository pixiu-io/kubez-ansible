#!/usr/bin/env bash

# NOTE(caoyingjun): Adjust the tag to keep consistence with
# kubernetes release. defualt: v1.16.2.
# Adjust the HOST_PORT if you wanted.

IMAGE_TAG=v1.16.2
HOST_PORT=4000

docker run -d \
    --name kube_registry \
    --restart=always \
    -p ${HOST_PORT}:5000 \
    jacky06/kube-registry:${IMAGE_TAG}
