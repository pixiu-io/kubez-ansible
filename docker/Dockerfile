FROM ubuntu:24.04

WORKDIR /kubez-ansible
COPY . .

RUN apt-get update
RUN apt install -y --no-install-recommends ansible sshpass && \
    pip install --no-cache-dir /kubez-ansible --break-system-packages && \
    apt autoremove -y && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

COPY docker/start.sh /usr/local/bin/pixiu_start

CMD ["pixiu_start"]
