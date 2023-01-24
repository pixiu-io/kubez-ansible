# Kubez-ansible Overview

Kubez-ansible's mission statement is:

``` bash
To provide quick deployment tools for kubernetes cluster.
```

This session has been tested on CentOS 7 and Ubuntu 18.04 which supported by
python2.7 for now.

#### 准备工作

- [前提条件](docs/source/install/prerequisites.md)

#### 环境部署

- [单节点部署](docs/source/install/all-in-one.md)

- [高可用环境](docs/source/install/multinode.md)

- [扩容](docs/source/install/expansion.md)

- [销毁](docs/source/install/destroy.md)

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
