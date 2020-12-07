#!/bin/bash
#
# This script can be used to install with harbor via ansible.


harbor=`helm list --kubeconfig /etc/kubernetes/admin.conf | grep harbor| grep -v grep |  awk '{print $1}'`
if [  "$harbor" != "" ] ; then
  echo 'harbor exits'
else
  helm install kube-harbor /tmp/kubez-ansible/ansible/roles/kubernetes/files/harbor-helm/ --kubeconfig /etc/kubernetes/admin.conf
fi
