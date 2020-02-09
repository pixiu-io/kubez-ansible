#!/usr/bin/env bash

# NTOE(caoyingjun): Ths script complete the helm3 installed for temp,
# but it will be optimise laster

IMAGE=$1

docker run -d --name helm_toolbox $IMAGE
docker cp helm_toolbox:/usr/bin/helm /usr/bin/helm
chmod +x /usr/bin/helm
docker rm helm_toolbox -f
