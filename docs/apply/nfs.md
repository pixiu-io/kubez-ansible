# NFS CSI 安装

### 依赖条件
- 运行正常的 `kubernetes` 环境。安装手册参考 [高可用集群](../install/multinode.md) 或 [单节点集群](../install/all-in-one.md)

### 开启 NFS 组件
1. 默认 `nfs csi` 是开启状态

2. 关闭 `nfs csi`
    ```shell
    # 取消 `enable_nfs_csi: "yes" 的注释, 并设置为 "no" 可关闭 nfs
    #######################
    # StorageClass Options
    #######################
    enable_nfs_csi: "no"
    ```

3. 部署完验证
   ```shell
   [root@pixiu]# kubectl get pod -n kube-system
   NAME                                     READY   STATUS    RESTARTS   AGE
   csi-nfs-controller-6b468b7554-dvnfc         3/3     Running            0                  9d
   csi-nfs-node-vfm8m                          3/3     Running            0                  9d

   [root@pixiu]# kubectl get storageclass
   NAME                  PROVISIONER      RECLAIMPOLICY   VOLUMEBINDINGMODE   ALLOWVOLUMEEXPANSION   AGE
   managed-nfs-storage   nfs.csi.k8s.io   Delete          Immediate           false                  9d

   [root@pixiu]# kubectl get csidriver
   NAME                  ATTACHREQUIRED   PODINFOONMOUNT   STORAGECAPACITY   TOKENREQUESTS   REQUIRESREPUBLISH   MODES                  AGE
   nfs.csi.k8s.io        false            false            false             <unset>         false               Persistent             9d
   ```
