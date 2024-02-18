FROM ubuntu:20.04

WORKDIR /kubez-ansible
COPY . .

RUN apt-get update
RUN apt install -y git python3-pip ansible && \
    pip3 install /kubez-ansible && \
    apt remove -y git python3-pip && \
    apt autoremove -y && \
    apt-get clean
