=========================================================
Kubernetes-ansible Overview  -- 让猩猩都能玩转kubernetes
=========================================================

Kubernetes-ansible's mission statement is:

::

    To provide quick deployment tools for kubernetes cluster.


=======================
测试环境部署 All-in-one
=======================

1. 执行如下命令完成all-in-one环境的快速安装

.. code-block:: ini

   curl https://raw.githubusercontent.com/yingjuncao/kubernetes-ansible/master/tools/all-in-one.sh | bash


.. code-block:: ini

   如果上述命令因为网络原因执行失败，拷贝``tools/all-in-one.sh`` 内容到本地，并执行.


2. (可选)：all-in-one部署也可以参考多节点的部署方式，部分步骤不需执行：

.. code-block:: ini

    a、不需要开通自身的免密登陆以及hosts的配置
    b、执行命令时，不需要加 ``-i multinude``


======================
生产环境部署 multinode
======================


部署前准备
----------

1. 安装部署节点的依赖,执行

.. code-block:: ini

   curl https://raw.githubusercontent.com/yingjuncao/kubernetes-ansible/master/tools/setup_env.sh | bash

.. code-block:: ini

   如果上述命令因为网络原因执行失败，拷贝``tools/setup_env.sh`` 内容到本地，并执行.


2. 编辑当前目录的 ``multinode`` ，完成主机组配置，手动开通部署节点到工作节点的免密登陆，并用如下命令测试

.. code-block:: ini

   ansible -i multinode all -m ping


开启私有仓库(可选)
------------

1. 拷贝 ``tools/setup_registry.sh`` script，到指定节点并运行，完成私有仓库的搭建

.. code-block:: ini

   a. 默认registry的版本: v1.17.2, 服务端口: 4000，可以根据实际情况修改
   b. 执行前，确保该节点docker服务处于正常运行状态
   c. 安装完成之后,运行 ``curl registry_server_ip:4000/v2/_catalog`` 确保registry运行正常

2. 配置 ``/etc/kubernetes-ansible/globals.yml``

.. code-block:: ini

   enable_registry: "yes"
   registry_server: ``registry_server_ip:4000``

3. 参考下文步骤，继续进行kubernetes集群的安装


kubernetes集群部署
------------------

1. 配置工作目录下的 ``multinode`` ,根据实际情况添加主机信息

.. code-block:: ini

   vim multinode

   [control]
   kube01

   [compute]
   kube02

2. 配置 ``/etc/kubernetes-ansible/globals.yml``

.. code-block:: ini

   enable_kubernetes_ha: "yes"  # (可选)如果启用多控，则开启, 并保证multunode的controller组为奇数

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


Apply kubernetes Applications
------------------------------

1. 执行如下命令

.. code-block:: ini

   kubernetes-ansible -i multinode apply


生成kubernetes admin-k8src.sh
------------------------------

1. 完成k8s的部署之后，需要导入KUBECONFIG到环境变量, 生成admin-k8src.sh

.. code-block:: ini

   kubernetes-ansible -i multinode post-deploy

2. 在master节点运行k8s集群命令

.. code-block:: ini

   . /root/admin-k8src.sh
   kubectl get node


kubernetes cluster node扩容
---------------------------

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


kubernetes 清理集群
-------------------

1. kubernetes清理

.. code-block:: ini

   kubernetes-ansible -i multinode destroy  --yes-i-really-really-mean-it

2. 如果环境允许，重启服务器，用来清除flannel.1和cni0的残留信息

.. code-block:: ini

   ansible -i multinode all -m shell -a reboot


安装Helm
---------

1. 配置 ``/etc/kubernetes-ansible/globals.yml``, 开启helm选项（默认关闭）

.. code-block:: ini

   enable_helm: "yes"

2. 执行 ``kubernetes-ansible apply`` 完成helm3的安装.

.. code-block:: ini

  （可选）：直接拷贝 ``tools/setup_helm.sh``到指定节点并允许, 完成helm的安装.


Ceph配置
--------

1. 登陆到ceph集群的monitor节点，为kubernetes创建pool和client auth(现假设pool name为kube)

.. code-block:: ini

   ceph osd pool create kube 8 8
   ceph auth add client.kube mon 'allow r' osd 'allow rwx pool=kube'

2. 获取ceph集群 ``admin`` 和新建pool ``kube`` 的auth key

.. code-block:: ini

   ceph auth get-key client.admin | base64 （记录回显值为admin_key，后续步骤需要用）
   ceph auth get-key client.kube | base64 （记录回显值为pool_key，后续步骤需要用）

3. 登陆到部署节点，编辑 ``/etc/kubernetes-ansible/globals.yml``

.. code-block:: ini

   enable_rbd_provisioner: "yes"

   pool_name: kube
   monitors: monitor_ip:port (port默认为6789)
   admin_key: admin_key
   pool_key: pool_key

4. 执行 ``kubernetes-ansible apply`` 完成external ceph集成.

.. code-block:: ini

   multinode场景需要加 ``-i multinode``执行

5. apply ``tools/test-rbd.yaml`` 进行测试，会达到类似如下回显

.. code-block:: ini

   [root@kube02 tools]# kubectl get pvc
   NAME       STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
   test-rbd   Bound    pvc-487cf629-24e8-4889-a977-dc8ac6c48d22   1Gi        RWO            rbd            25m

   [root@ceph-monitor ~]# rbd ls kube
   kubernetes-dynamic-pvc-d4a56035-4a94-11ea-aa72-d23b78a708e0
