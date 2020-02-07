===========================
Kubernetes-ansible Overview
===========================

Kubernetes-ansible's mission statement is:

::

    To provide quick deployment tools for kubernetes cluster.


==========
部署前准备
==========

1. 安装部署节点的依赖,执行

.. code-block:: ini

   curl https://raw.githubusercontent.com/yingjuncao/kubernetes-ansible/master/tools/setup_env.sh | bash

.. note::

   如果上述命令因为网络原因执行失败，拷贝``tools/setup_env.sh`` 内容到本地，并执行.



2. 编辑当前目录的 ``multinode`` ，完成主机组配置，手动开通部署节点到工作节点的免密登陆，并用如下命令测试

.. code-block:: ini

   ansible -i multinode all -m ping

==================
kubernetes集群部署
==================

1. 配置工作目录下的 ``multinode`` ,根据实际情况添加主机信息

.. code-block:: ini

   vim multinode

   [control]
   kube01

   [compute]
   kube02

2. 配置 ``/etc/kubernetes-ansible/globals.yml``

.. code-block:: ini

   cluster_cidr: "172.30.0.0/16"
   service_cidr: "10.254.0.0/16"

3. 配置网络插件，目前已经支持的网络CNI有三种：flannel, calico, 和ovn, 默认是flannel.
   如果开启其他CNI的话，需要在 ``/etc/kubernetes-ansible/globals.yml`` 中添加配置

.. code-block:: ini

   enable_calico: "yes" or enable_ovn: "yes"

4. 安装kubernetes依赖包

.. code-block:: ini

   kubernetes-ansible -i multinode bootstrap-servers

5. 进行kubernetes的部署

.. code-block:: ini

   kubernetes-ansible -i multinode deploy

=============================
Apply kubernetes Applications
=============================

1. 执行如下命令

.. code-block:: ini

   kubernetes-ansible -i multinode apply

=============================
生成kubernetes admin-k8src.sh
=============================

1. 完成k8s的部署之后，需要导入KUBECONFIG到环境变量, 生成admin-k8src.sh

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

============
开启私有仓库
============

1. 配置 ``/etc/kubernetes-ansible/globals.yml``

.. code-block:: ini

   enable_registry: "yes"

2. 编辑 ``multinode`` , 在registry组配置节点，完成之后，该节点将作为私有仓库运行节点.

.. code-block:: ini

   [registry]
   control01

.. note::

   registry repository: https://hub.docker.com/repository/docker/jacky06/kube-registry

========
安装Helm
========

1. 运行 ``tools/setup_helm.sh``, 完成helm的安装.
