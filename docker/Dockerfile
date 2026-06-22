FROM ubuntu:24.04

WORKDIR /kubez-ansible
COPY . .

RUN apt-get update && \
    apt-get install -y --no-install-recommends sshpass && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# pip 安装 ansible 和项目（自动安装 ansible）
RUN pip install --no-cache-dir ansible /kubez-ansible --break-system-packages

COPY docker/start.sh /usr/local/bin/pixiu_start

CMD ["pixiu_start"]
