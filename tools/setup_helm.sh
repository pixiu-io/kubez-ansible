#!/usr/bin/env bash

docker run -d --name helm_toolbox jacky06/helm-toolbox:v3.0.3
docker cp helm_toolbox:/usr/bin/helm /usr/bin/helm
chmod +x /usr/bin/helm
docker rm helm_toolbox -f
