/*
Copyright 2026 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apps

import (
	"fmt"
	"net/http"
	"strings"

	"helm.sh/helm/v3/pkg/chart/loader"
	helmchartutil "helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
)

// clusterScopedKinds 明确为集群级（cluster-scoped）或高风险的 Kubernetes 资源类型。
// 命中即拒绝安装（管理员可豁免）。
var clusterScopedKinds = map[string]struct{}{
	"ClusterRole":                      {},
	"ClusterRoleBinding":               {},
	"CustomResourceDefinition":         {},
	"Namespace":                        {},
	"PersistentVolume":                 {},
	"StorageClass":                     {},
	"ValidatingWebhookConfiguration":   {},
	"MutatingWebhookConfiguration":     {},
	"APIService":                       {},
	"IngressClass":                     {},
	"PriorityClass":                    {},
	"RuntimeClass":                     {},
	"PodSecurityPolicy":                {},
	"CSIDriver":                        {},
	"CSINode":                          {},
	"VolumeAttachment":                 {},
	"Node":                             {},
	"CertificateSigningRequest":        {},
	"TokenReview":                      {},
	"SubjectAccessReview":              {},
	"ClusterIssuer":                    {},
	"ValidatingAdmissionPolicy":        {},
	"ValidatingAdmissionPolicyBinding": {},
}

// namespacedKinds 常见命名空间级资源类型：渲染后未显式写 metadata.namespace 属正常（helm 安装时注入目标命名空间）。
// 不在本清单、且未显式指定 namespace 的资源，视为作用域未知，保守拒绝。
var namespacedKinds = map[string]struct{}{
	"Deployment":              {},
	"StatefulSet":             {},
	"DaemonSet":               {},
	"ReplicaSet":              {},
	"ReplicationController":   {},
	"Pod":                     {},
	"Job":                     {},
	"CronJob":                 {},
	"Service":                 {},
	"ConfigMap":               {},
	"Secret":                  {},
	"ServiceAccount":          {},
	"Role":                    {},
	"RoleBinding":             {},
	"Ingress":                 {},
	"NetworkPolicy":           {},
	"Endpoints":               {},
	"PersistentVolumeClaim":   {},
	"PodDisruptionBudget":     {},
	"HorizontalPodAutoscaler": {},
	"LimitRange":              {},
	"ResourceQuota":           {},
	"ControllerRevision":      {},
	"Event":                   {},
	"PodTemplate":             {},
	"ServiceEntry":            {},
}

// guard 安全预检：渲染 chart 并拒绝 cluster-scoped 资源。
// 判定规则：① kind 命中集群级黑名单；② 资源未显式声明 metadata.namespace 且 kind 不在常见命名空间级白名单内（作用域未知，保守拒绝）。
// 管理员（RoleRoot / RoleAdmin）豁免本预检：应用商店允许管理员安装需要集群级权限的 chart（如 ingress controller）。
//
// 异步化后 worker 无请求用户上下文，故 isAdmin 由调用方依据记录的 created_by 用户 role 预先判定后传入。
func (c *controller) guard(chartPath, releaseName, namespace string, values map[string]interface{}, isAdmin bool) error {
	if isAdmin {
		return nil
	}

	ch, err := loader.Load(chartPath)
	if err != nil {
		return apierrors.NewError(fmt.Errorf("failed to load chart: %w", err), http.StatusBadRequest)
	}

	renderVals, err := helmchartutil.ToRenderValues(ch, values, helmchartutil.ReleaseOptions{
		Name:      releaseName,
		Namespace: namespace,
		IsInstall: true,
		Revision:  1,
	}, nil)
	if err != nil {
		return apierrors.NewError(fmt.Errorf("failed to render chart values: %w", err), http.StatusBadRequest)
	}

	rendered, err := engine.Render(ch, renderVals)
	if err != nil {
		return apierrors.NewError(fmt.Errorf("failed to render chart: %w", err), http.StatusBadRequest)
	}

	for name, content := range rendered {
		base := name
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			base = name[idx+1:]
		}
		// 跳过非资源文件
		if strings.HasSuffix(base, "NOTES.txt") || strings.HasPrefix(base, "_") {
			continue
		}

		decoder := utilyaml.NewYAMLOrJSONDecoder(strings.NewReader(content), 4096)
		for {
			var obj unstructured.Unstructured
			if err := decoder.Decode(&obj); err != nil {
				// 读取完毕或该文档无法解析（如局部模板片段）时停止
				break
			}
			if len(obj.Object) == 0 {
				continue
			}
			kind := obj.GetKind()
			if kind == "" {
				continue
			}
			if _, banned := clusterScopedKinds[kind]; banned {
				return apierrors.NewError(
					fmt.Errorf("chart contains cluster-scoped resource %s/%s, which is not allowed for non-admin install", kind, obj.GetName()),
					http.StatusForbidden,
				)
			}
			if obj.GetNamespace() == "" {
				if _, known := namespacedKinds[kind]; !known {
					return apierrors.NewError(
						fmt.Errorf("chart resource %s/%s has no namespace and its scope is unknown, refusing to install", kind, obj.GetName()),
						http.StatusForbidden,
					)
				}
			}
		}
	}

	return nil
}
