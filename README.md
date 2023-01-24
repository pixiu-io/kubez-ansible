# Kubez-ansible Overview

kubez-ansible's mission statement is:

``` bash
To provide quick deployment tools for kubernetes cluster.
```

This session has been tested on CentOS 7 and Ubuntu 18.04 which supported by
python2.7 for now.

### 环境类型

|  [单节点集群](docs/install/all-in-one.md) | [高可用集群](docs/install/multinode.md)  |
|  :----:  | :----:  |

### 场景操作
|  [集群扩容](docs/install/expansion.md) | [集群销毁](docs/install/destroy.md)  |
|  :----:  | :----:  |

### 组件支持清单

#### CNI
  - `Flannel`
  - `Calico`

#### CRI
  - `docker`
  - `containerd`

#### Base Applications
  - `Helm3`
  - `Nginx Ingress`
  - `Ceph provisioner`
  - `Nfs provisioner`
  - `Grafana`
  - `Prometheus`
  - `OLM`
  - `Dashboard`
  - `Kibana`
  - `Fluentd`
  - `Filebeat`
  - `Elasticsearch`
  - `Pixiu-autoscaler`

#### Addons Applications
  - `Isito`
  - `Jenkins`
  - `Harbor`
  - `Consul`
  - `Mariadb`
  - `Kong`
  - `Redis`

Copyright 2019 caoyingjun (cao.yingjunz@gmail.com) Apache License 2.0
