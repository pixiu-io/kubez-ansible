FROM ubuntu:24.04

WORKDIR /kubez-ansible
COPY . .

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        git python3-pip ansible-core sshpass && \
    pip3 install --no-cache-dir /kubez-ansible --break-system-packages && \
    ansible-galaxy collection install ansible.posix:1.5.4 community.general:8.3.0 \
        -p /usr/share/ansible/collections && \
    apt-get purge -y git python3-pip && \
    apt-get autoremove --purge -y && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /root/.cache /root/.ansible /kubez-ansible && \
    mkdir -p /kubez-ansible

COPY docker/start.sh /usr/local/bin/pixiu_start

CMD ["pixiu_start"]
