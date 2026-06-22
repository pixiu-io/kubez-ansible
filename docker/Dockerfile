FROM python:3.12-slim

WORKDIR /kubez-ansible
COPY . .

# 安装系统依赖（sshpass）和项目
RUN apt-get update && \
    apt-get install -y --no-install-recommends sshpass && \
    pip install --no-cache-dir ansible /kubez-ansible --break-system-packages && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

COPY docker/start.sh /usr/local/bin/pixiu_start

CMD ["pixiu_start"]
