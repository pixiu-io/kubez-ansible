FROM ubuntu:20.04

WORKDIR /root
COPY . .

RUN apt-get update && \
    apt install -y git python3-pip ansible && \
    pip3 install /root/kubez-ansible && \
    apt remove -y git python3-pip && \
    rm -rf /root/kubez-ansible && \
    apt autoremove -y && \
    apt-get clean
