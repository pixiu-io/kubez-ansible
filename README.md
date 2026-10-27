# Kubez-ansible Overview

To provide quick deployment tools for kubernetes cluster and cloud native application

![Build Status][build-url]
[![Release][release-image]][release-url]
[![License][license-image]][license-url]

This session has been tested on Rocky 8.5+, AlmaLinux 9.0+, Debian 11+, Ubuntu 20.04+, openEuler 22.03+ and Kylin V10+ which supported by python3.

More distribution supported see [more](https://github.com/gopixiu-io/kubez-ansible/tree/stable/tiger)

## Getting Started
Learn about Kubez Ansible by reading the documentation online [kubez-ansible](https://www.bilibili.com/video/BV1L84y1h7LE/).

## Supported Components
- 集群指南
  - [单节点集群](docs/install/all-in-one.md) 单节点集群的快速部署
  - [多节点集群](docs/install/multinode.md) 多节点集群部署 (1master + 多node)
  - [高可用集群](docs/install/availability.md) 高可用集群部署 (3master + 多node)
  - [扩容](docs/install/expansion.md)
  - [销毁](docs/install/destroy.md)

- 网络插件
  - [flannel](https://github.com/flannel-io/flannel)
  - [calico](https://github.com/projectcalico/calico)
  - [cilium](https://github.com/cilium/cilium)

- 容器运行时
  - [containerd](https://github.com/containerd/containerd)

- 应用安装
  - [应用商店操作指南](https://github.com/caoyingjunz/pixiu/blob/master/docs/appstore/README.md) 支持 Pixiu 官方内置和自定义应用的上架，安装，升级和卸载

- 自研解决方案
  - [Pixiu](https://github.com/caoyingjunz/pixiu)

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
