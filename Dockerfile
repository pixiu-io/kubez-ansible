FROM ubuntu:20.04

WORKDIR /kubez-ansible
COPY . .

RUN apt-get update
RUN apt install -y git python-pip ansible && \
    pip install /kubez-ansible && \
    apt remove -y git python-pip && \
    apt autoremove -y && \
    apt-get clean
