FROM ubuntu:20.04

WORKDIR /kubez-ansible
COPY . .

RUN apt-get update
RUN apt install -y git python3-pip sshpass && \
    pip3 install ansible==2.12.2 && \
    pip3 install /kubez-ansible && \
    ansible --version && \
    apt-get clean

COPY docker/start.sh /usr/local/bin/pixiu_start

CMD ["pixiu_start"]
