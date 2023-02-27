# Kubez-ansible Overview

kubez-ansible's mission statement is

``` bash
To provide quick deployment tools for kubernetes cluster.
```

This session has been tested on CentOS 7 and Ubuntu 18.04 which supported by
python2.7 for now.

### 源码分析
[源码分析](https://www.bilibili.com/video/BV1L84y1h7LE/)

### 环境类型
| [单节点集群](docs/install/all-in-one.md) | [多节点&高可用集群](docs/install/multinode.md) | [集群扩容](docs/install/expansion.md) | [集群销毁](docs/install/destroy.md) |
| :----: | :----: | :----:  | :----: |

### 容器&网络
| [Docker](https://github.com/docker) | [Containerd](https://github.com/containerd/containerd) | [Flannel](https://github.com/flannel-io/flannel) | [Calico](https://github.com/projectcalico/calico) |
| :----: | :----: | :----: | :----: |

### 存储
| 文件存储 [NFS](docs/apply/nfs.md) | 块存储 [Ceph](docs/apply/ceph-guide.md) | 对象存储 [MinIO](docs/apply/minio.md)|
| :----: | :----: | :----: |

### 云原生组件

#### 基础组件
| [Helm3](docs/apply/helm3-guide.md) | [Nginx Ingress](docs/apply/ingress.md) | [Dashboard](docs/apply/dashboard.md) | [Metrics Server](docs/apply/metrics.md) |
| :----: | :----: | :----: | :----: | 

#### 日志&监控
| [Loki](docs/apply/loki.md) | [Grafana](docs/apply/grafana.md) | [Promtail](docs/apply/promtail.md) | [Prometheus](docs/apply/prometheus.md) |
| :----: | :----: | :----: | :----: |

#### 中间件
| [OLM](docs/paas/olm.md) | [Postgres](docs/paas/postgres.md)  | [Redis](docs/paas/redis.md) | [Kafka](docs/paas/kafka.md) | [RabbitMQ](docs/paas/rabbitmq.md) | [MongoDB](docs/paas/mongodb.md) |
| :----: | :----: | :----: | :----: | :----: | :----: |

#### 微服务&DevOps
| Isito | [Jenkins](docs/apply/jenkins.md) | [Harbor](docs/apply/harbor.md) |
| :----: |:----:|:----:|

### 自研 Kubernetes 原生功能强化项目
| [Pixiu](https://github.com/caoyingjunz/pixiu) | [Pixiu-autoscaler](https://github.com/caoyingjunz/pixiu-autoscaler) | [PodSet](https://github.com/caoyingjunz/podset-operator) |
| :----: | :----: | :----: |

### 学习分享
| [go-learning](https://github.com/caoyingjunz/go-learning) |
| :----: |

### 沟通交流
- 搜索微信号 `yingjuncz`, 备注（github）, 验证通过会加入群聊
- [bilibili](https://space.bilibili.com/3493104248162809?spm_id_from=333.1007.0.0) 技术分享

Copyright 2019 caoyingjun (cao.yingjunz@gmail.com) Apache License 2.0
