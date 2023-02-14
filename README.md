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
| [单节点集群](docs/install/all-in-one.md) | [高可用集群](docs/install/multinode.md) | [集群扩容](docs/install/expansion.md) | [集群销毁](docs/install/destroy.md) |
| :----: | :----: | :----:  | :----: |

### 运行时
| [docker](https://github.com/docker) | [containerd](https://github.com/containerd/containerd) |
| :----: | :----: |

### 容器网络
| [Flannel](https://github.com/flannel-io/flannel) | [Calico](https://github.com/projectcalico/calico) |
| :----: | :----: |

### 基础组件
| [Helm3](docs/apply/helm3-guide.md) | [Nginx Ingress](docs/apply/ingress.md) | [Dashboard](docs/apply/dashboard.md) | Metrics Server | [NFS](docs/apply/nfs.md) | Ceph |
| :----: | :----: | :----: | :----: | :----: | :----: |

### 云原生组件
#### 日志
| Elasticsearch | Kibana | Filebeat | Fluentd |
| :----: | :----: | :----:  | :----: |

#### 中间件
| [OLM](docs/paas/olm.md) | [Postgres](docs/paas/postgres.md) | [Mariadb](docs/paas/mariadb.md) | [Redis](docs/paas/redis.md) | [Kafka](docs/paas/kafka.md) | [RabbitMQ](docs/paas/rabbitmq.md) | [MongoDB](docs/paas/mongodb.md)
| :----: | :----: | :----: | :----: | :----: | :----: | :----: |

#### 监控
| [Prometheus](docs/apply/prometheus.md) | [Grafana](https://grafana.com/docs/grafana/latest/whatsnew) |
| :----: | :----: |

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
