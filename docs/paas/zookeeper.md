# Zookeeper 安装

### 依赖条件
- 运行正常的 `kubernetes` ( v1.21+ )环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)
- StorageClass

### 开启 Zookeeper 组件
1. 编辑 `/etc/kubez/globals.yml`

2. 取消 `enable_zookeeper: "no"` 的注释，设置为 `"yes"`，并取消如下参数注释
    ```shell
   ######################
   # zookeeper
   ######################
   enable_zookeeper: "yes"
   # helm 配置
   #zookeeper_repository_name: "{{ default_repo_name }}"
   #zookeeper_repository_url: "{{ default_repo_url }}"
   #zookeeper_repository_path: "{{ default_repo_name }}/zookeeper"
   # zookeeper配置
   zookeeper_replicas: 3
   #zookeeper_requests_cpu: 1
   #zookeeper_requests_memory: "1Gi"
   #zookeeper_namespace: "{{ kubez_namespace }}"
   #zookeeper_image_registry: "{{ app_image_repository }}"
   #zookeeper_image_repository: "zookeeper"
   #zookeeper_version: "11.4.9"
   # zookeeper持久化设置
   zookeeper_persistence_enabled: "true"
   #zookeeper_persistence_size: "8Gi"
   #zookeeper_persistence_storageclass: "{{ nfs_storage_class }}"
   #zookeeper_context_fsgroup: "0"
    ```
3. 执行安装命令（根据实际情况选择）
    ```shell
    # 单节点集群场景
    kubez-ansible apply

    # 高可用集群场景
    kubez-ansible -i multinode apply
    ```
4. 部署完验证
    ```shell
   root@VM-0-16-ubuntu:/home/ubuntu# kubectl get pod -n pixiu-system
   zookeeper-0                                 1/1     Running            0          16s
   zookeeper-1                                 1/1     Running            0          16s
   zookeeper-2                                 1/1     Running            0          16s
   root@VM-0-16-ubuntu:/home/ubuntu# kubectl get pvc -n pixiu-system
   NAME               STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          AGE
   data-zookeeper-0   Bound    pvc-9723f14b-e87c-4276-ba6a-55497068757e   8Gi        RWO            managed-nfs-storage   3m23s
   data-zookeeper-1   Bound    pvc-5c73b82c-0312-4c67-8929-037d8fc1b0bb   8Gi        RWO            managed-nfs-storage   19s
   data-zookeeper-2   Bound    pvc-1b6e23f0-0f90-496a-a7ec-433796b5d06d   8Gi        RWO            managed-nfs-storage   19s
   ```
至此 `zookeeper` 已安装至集群中。