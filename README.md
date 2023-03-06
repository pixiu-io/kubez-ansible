
# Kubez-ansible Overview

To provide quick deployment tools for kubernetes cluster and cloud native application

![Build Status][build-url]
[![Release][release-image]][release-url]
[![License][license-image]][license-url]

This session has been tested on CentOS 7 and Ubuntu 18.04 which supported by python2.7 for now.

## Getting Started
Learn about Kubez Ansible by reading the documentation online [kubez-ansible](https://www.bilibili.com/video/BV1L84y1h7LE/).

## Supported Components
- 集群指南
  - [单节点部署](docs/install/all-in-one.md) 单节点集群的快速部署
  - [高可用集群](docs/install/multinode.md) 多节点和高可用集群搭建的文档说明
  - [扩容](docs/install/expansion.md) `kubernetes` 集群节点扩容
  - [销毁](docs/install/destroy.md) 重置 `kubernetes` 集群
  
- 网络插件
  - [flannel](https://github.com/flannel-io/flannel)
  - [calico](https://github.com/projectcalico/calico)

- 容器运行时
  - [docker](https://github.com/docker)
  - [containerd](https://github.com/containerd/containerd)

- 存储插件
  - [NFS](docs/apply/nfs.md) 文件存储
  - [Ceph](docs/apply/ceph-guide.md) 块存储
  - [MinIO](docs/apply/minio.md) 对象存储

- 云原生应用
  - [Helm3](docs/apply/helm3-guide.md)
  - [Nginx Ingress](docs/apply/ingress.md)
  - [Dashboard](docs/apply/dashboard.md)
  - [Metrics Server](docs/apply/metrics.md)
  - [Loki](docs/apply/loki.md)
  - [Grafana](docs/apply/grafana.md)
  - [Promtail](docs/apply/promtail.md)
  - [Prometheus](docs/apply/prometheus.md)
  - [OLM](docs/paas/olm.md)
  - [PostgreSQL](docs/paas/postgres.md)
  - [Redis](docs/paas/redis.md)
  - [Kafka](docs/paas/kafka.md)
  - [RabbitMQ](docs/paas/rabbitmq.md)
  - [MongoDB](docs/paas/mongodb.md)
  - [Isito]()
  - [Jenkins](docs/apply/jenkins.md)
  - [Harbor](docs/apply/harbor.md)

- 自研云原生
  - [Pixiu](https://github.com/caoyingjunz/pixiu)
  - [Pixiu-autoscaler](https://github.com/caoyingjunz/pixiu-autoscaler)
  - [PodSet](https://github.com/caoyingjunz/podset-operator)
  
## 学习分享
- [go-learning](https://github.com/caoyingjunz/go-learning)

## 沟通交流
- 搜索微信号 `yingjuncz`, 备注（github）, 验证通过会加入群聊
- [bilibili](https://space.bilibili.com/3493104248162809?spm_id_from=333.1007.0.0) 技术分享

Copyright 2019 caoyingjun (cao.yingjunz@gmail.com) Apache License 2.0

[build-url]: https://github.com/gopixiu-io/kubez-ansible/actions/workflows/ci.yml/badge.svg
[release-image]: https://img.shields.io/badge/release-download-orange.svg
[release-url]: https://www.apache.org/licenses/LICENSE-2.0.html
[license-image]: https://img.shields.io/badge/license-Apache%202-4EB1BA.svg
[license-url]: https://www.apache.org/licenses/LICENSE-2.0.html
