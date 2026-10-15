/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (phe "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

const GlobalsTemplate = `# Render below by Pixiu
---
{{- if .Kubernetes.EnableHA }}
enable_kubernetes_ha: "yes"
{{- end }}

{{- if .Kubernetes.EnablePublicIp }}
kube_vip_address: "{{ .Kubernetes.ApiServer }}"
{{- end }}

{{- if .Kubernetes.ApiPort }}
kube_vip_port: "{{ .Kubernetes.ApiPort }}"
{{- end }}

kube_release: {{ .Kubernetes.KubernetesVersion }}

cluster_cidr: "{{ .Network.PodNetwork }}"
service_cidr: "{{ .Network.ServiceNetwork }}"

network_interface: "{{ .Network.NetworkInterface }}"

{{- if eq .Network.KubeProxy "ipvs" }}
kube_proxy_mode: "ipvs"
{{- end }}

{{- if and .Component.Haproxy .Component.Haproxy.Enable }}
enable_haproxy: "yes"
{{- if .Component.Haproxy.KeepalivedVirtualRouterId }}
keepalived_virtual_router_id: "{{ .Component.Haproxy.KeepalivedVirtualRouterId }}"
{{- end }}
{{- end }}

cni: "{{ .Network.Cni }}"

# 自定义配置
{{- if ne .Kubernetes.ImageRepository "" }}
image_repository: "{{ .Kubernetes.ImageRepository }}"
app_image_repository: "{{ .Kubernetes.ImageRepository }}"
{{- end }}

{{- if and .Component.MetricServer .Component.MetricServer.Enable }}
enable_metrics_server: "yes"
enable_metrics_scraper: "no"
{{- else }}
enable_metrics_server: "no"
enable_metrics_scraper: "no"
{{- end }}

{{- if and .Component.IngressNginx .Component.IngressNginx.Enable }}
enable_ingress_nginx: "yes"
{{- else }}
enable_ingress_nginx: "no"
{{- end }}

{{- if ne .Runtime.DataDir "" }}
runtime_data_dir: "{{ .Runtime.DataDir }}"
{{- end }}

{{- if .Kubernetes.SetHostname }}
set_hostname: "yes"
{{- else }}
set_hostname: "no"
{{- end }}

{{- if not .Kubernetes.ChangeSelinux }}
change_selinux: "no"
{{- end }}

{{- if and .Component.NFS .Component.NFS.Enable }}
enable_nfs: "yes"

{{- if ne .Component.NFS.StorageClassName "" }}
nfs_storage_class: "{{ .Component.NFS.StorageClassName }}"
{{- end }}
{{- if ne .Component.NFS.StorageDataDir "" }}
nfs_share: "{{ .Component.NFS.StorageDataDir }}"
{{- end }}

{{- else }}
enable_nfs: "no"
{{- end }}

{{- if and .Component.CustomRepo .Component.CustomRepo.Enable }}
enable_custom_repo: "yes"
{{- end }}

{{- if and .Component.CertificatePeriod .Component.CertificatePeriod.Enable }}
enable_certificate_period: "yes"
certificate_validity_period: {{ .Component.CertificatePeriod.CertificateValidityPeriod }}h
ca_certificate_validity_period: {{ .Component.CertificatePeriod.CaCertificateValidityPeriod }}h
{{- end }}

{{- range .Component.CustomConfigs }}
{{- if and (ne .Key "") (ne .Value "") }}
{{ .Key }}: {{ printf "%q" .Value }}
{{- end }}
{{- end }}

# 组件默认开关
enable_prometheus: "no"
enable_prometheus_adapter: "no"
enable_pixiu_autoscaler: "no"
enable_helm: "no"
`
