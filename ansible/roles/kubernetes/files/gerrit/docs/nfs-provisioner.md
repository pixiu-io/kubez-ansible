# Installing a NFS-provisioner

Gerrit requires access to a persistent volume capable of running in
`Read Write Many (RWM)`-mode to store the git repositories, since the repositories
have to be accessed by mutiple pods. One possibility to provide such volumes
is to install a provisioner for NFS-volumes into the same Kubernetes-cluster.
This document will guide through the process.

The [Kubernetes external-storage project](https://github.com/kubernetes-incubator/external-storage)
provides an out-of-tree dynamic [provisioner](https://github.com/kubernetes-incubator/external-storage/tree/master/nfs)
for NFS volumes. A chart exists for easy deployment of the project onto a
Kubernetes cluster. The chart's sources can be found [here](https://github.com/helm/charts/tree/master/stable/nfs-server-provisioner).

## Prerequisites

This guide will use Helm to install the NFS-provisioner. Thus, Helm has to be
installed.

## Installing the nfs-server-provisioner chart

A custom `values.yaml`-file containing a configuration tested with the
gerrit charts can be found in the `supplements/nfs`-directory in the
gerrit chart's root directory. In addition a file stating the tested
version of the nfs-server-provisioner chart is present in the same directory.

If needed, adapt the `values.yaml`-file for the nfs-server-provisioner chart
further and then run:

```sh
cd $(git rev-parse --show-toplevel)/helm-charts/gerrit/supplements/nfs
helm install nfs \
  stable/nfs-server-provisioner \
  -f values.yaml \
  --version $(cat VERSION)
```

For a description of the configuration options, refer to the
[chart's documentation](https://github.com/helm/charts/blob/master/stable/nfs-server-provisioner/README.md).

Here are some tips for configuring the nfs-server-provisioner chart to work with
the gerrit chart:

- Deploying more than 1 `replica` led to some reliability issues in tests and
  should be further tested for now, if required.
- The name of the StorageClass created for NFS-volumes has to be the same as the
  one defined in the gerrit chart for `storageClasses.shared.name`
- The StorageClas for NFS-volumes needs to have the parameter `mountOptions: vers=4.1`,
  due to compatibility [issues](https://github.com/kubernetes-incubator/external-storage/issues/223)
  with Ganesha.

## Deleting the nfs-server-provisioner chart

***note
**Attention:** Never delete the nfs-server-provisioner chart, if there is still a
PersistentVolumeClaim and Pods using a NFS-volume provisioned by the NFS server
provisioner. This will lead to crashed pods, that will not be terminated correctly.
***

If no Pod or PVC is using a NFS-volume provisioned by the NFS server provisioner
anymore, delete it like any other chart:

```sh
helm delete nfs
```
