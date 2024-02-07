# 设置基础镜像为最新版的alpine
FROM alpine:latest

# 设置镜像的维护者信息
LABEL maintainer="tyvekzhang <tyvekzhang@gmail.com>"

# 设置环境变量，这些环境变量在容器运行时可以被使用
ENV KEEP_NUM=3
ENV ETCD_ENDPOINTS=https://127.0.0.1:2379
ENV ETCD_CACERT=/etc/kubernetes/pki/etcd/ca.crt
ENV ETCD_CERT=/etc/kubernetes/pki/etcd/server.crt
ENV ETCD_KEY=/etc/kubernetes/pki/etcd/server.key
ENV BACKUP2MINIO=0
ENV S3_ENDPOINT=http://127.0.0.1:9000
ENV S3_BUCKET=etcd-backups
ENV S3_ACCESS_KEY=jm7GqGEbM0OpiqZo
ENV S3_SECRET_KEY=E6z4q5v28VXve9Ic9wHiw7TIYVrd8scB
ENV ETCD_VER=v3.5.11
ARG ETCD_DOWNLOAD_URL=https://github.com/etcd-io/etcd/releases/download
ENV ETCD_DOWNLOAD_URL=${ETCD_DOWNLOAD_URL}
ARG MC_DOWNLOAD_URL=https://dl.min.io/client/mc/release/linux-amd64/mc
ENV MC_DOWNLOAD_URL=${MC_DOWNLOAD_URL}
ENV BACKUP_DIR=/var/backups
ENV BACKUP_SCRIPTS_DIR=/usr/local/bin
ENV TZ=Asia/Shanghai

# 安装curl,tar和tzdata包，下载etcd，解压到指定路径，并设置执行权限, 最后设置时区
RUN set -ex \
    && apk add --no-cache curl tar tzdata \
    && mkdir -p /tmp/etcd-download-dir \
    && curl -L ${ETCD_DOWNLOAD_URL}/${ETCD_VER}/etcd-${ETCD_VER}-linux-amd64.tar.gz -o /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz \
    && tar xvf /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz -C /tmp/etcd-download-dir --strip-components=1 \
    && mv /tmp/etcd-download-dir/etcdctl ${BACKUP_SCRIPTS_DIR} \
    && chmod +x ${BACKUP_SCRIPTS_DIR}/etcdctl \
    && rm -f /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz \
    && rm -rf /tmp/etcd-download-dir \
    && rm -rf /var/cache/apk/* \
    && ln -snf /usr/share/zoneinfo/${TZ} /etc/localtime && echo ${TZ} > /etc/timezone

# 下载MinIO客户端，设置执行权限，并移动到指定目录下
RUN wget ${MC_DOWNLOAD_URL} && \
    chmod +x mc && \
    mv mc ${BACKUP_SCRIPTS_DIR}
