==================
kubernetes-ansible
==================


通用配置
--------

1. 安装部署节点的依赖,执行

.. code-block:: ini

   curl https://raw.githubusercontent.com/yingjuncao/kubernetes-ansible/master/tools/setup_env.sh | bash

2. 编辑当前目录的multinode，完成主机组配置，手动开通部署节点到工作节点的免密登陆，并用如下命令测试

.. code-block:: ini

   ansible -i multinode all -m ping

==================
kubernetes集群部署
==================

1. 配置工作目录下的multinode,根据实际情况添加主机信息

.. code-block:: ini

   vim multinode
   
   [control]
   kube01

   [compute]
   kube02

2. 配置/etc/kubernetes-ansible/globals.yml

.. code-block:: ini

   cluster_cidr: "172.30.0.0/16"
   service_cidr: "10.254.0.0/16"

3. 安装kubernetes依赖包

.. code-block:: ini

   kubernetes-ansible -i multinode bootstrap-servers

4. 进行kubernetes的部署

.. code-block:: ini

   kubernetes-ansible -i multinode deploy

=============================
生成kubernetes admin-k8src.sh
=============================

1. 完成k8s的部署之后，需要导入KUBECONFIG到环境变量（类似openstack), 生成admin-k8src.sh

.. code-block:: ini
   kubernetes-ansible -i multinode post-deploy

2. 在master节点运行k8s集群命令

.. code-block:: ini
   . /root/admin-k8src.sh
   kubectl get node

===========================
kubernetes cluster node扩容
===========================

1. 配置工作目录下的multinode,根据实际情况添加worker node到compute组

.. code-block:: ini

   vim multinode
   
   [control]
   kube1

   [compute]
   kube[2:4]
   
3. 安装worker node的依赖包

.. code-block:: ini

   kubernetes-ansible -i multinode bootstrap-servers

4. 进行worker node节点的扩容

.. code-block:: ini

   kubernetes-ansible -i multinode deploy

===================
kubernetes 清理集群
===================

1. kubernetes清理

.. code-block:: ini

   kubernetes-ansible -i multinode destroy  --yes-i-really-really-mean-it

2. 如果环境允许，重启服务器，用来清除flannel.1和cni0的残留信息

.. code-block:: ini

   ansible -i multinode all -m shell -a reboot

===========
开启私有仓库
===========

1. 配置/etc/kubernetes-ansible/globals.yml

.. code-block:: ini

   enable_registry: "yes"

2. 编辑multinode, 在registry组配置节点，完成之后，该节点将作为私有仓库运行节点.

.. code-block:: ini

   [registry]
   control01

.. note::
   registry仓库压缩包: https://hub.docker.com/repository/docker/jacky06/kube-registry

.. code-block:: ini

   在未开启私有仓库的情况下，kubernetes集群将从官方仓库获取镜像.
   在开启私有仓库，且联网的情况下，将自动下载并设置私有仓库，kubernetes集群优先从私有仓库获取所需镜像.
   在未联网时，需要手动获取registry压缩包，并放在/tmp下，然后自动完成私有仓库的设置，并使用私有仓库镜像(开发中).