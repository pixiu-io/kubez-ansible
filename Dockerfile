FROM ubuntu:18.04

WORKDIR /kubez-ansible
COPY . .

RUN apt-get update
RUN apt install -y git python-pip ansible && \
    pip install /kubez-ansible && \
    apt remove -y git python-pip && \
    apt-get clean
