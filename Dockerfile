FROM ubuntu:20.04

WORKDIR /kubez-ansible
COPY . .

RUN apt-get update
RUN apt install -y python3-pip ansible && pip3 install /kubez-ansible && apt-get clean
