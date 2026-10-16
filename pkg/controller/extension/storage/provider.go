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

package storage

import (
	"net"
	"sort"
	"strings"

	"github.com/minio/minio-go/v7"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

// 对象存储厂商的统一抽象：主流程只依赖 Provider 接口，厂商差异由各自的实现文件声明。
// 新增一家 = 新增一个实现文件 + 在 providers 里登记一行。

// Capabilities 描述一家厂商在控制面上的能力子集；S3 兼容端点的能力并不一致，按厂商声明。
type Capabilities struct {
	// BucketPolicy 是否支持通用 S3 Bucket 策略文档
	BucketPolicy bool
	// SSEEncryption 是否支持标准 SSE-S3 加密配置
	SSEEncryption bool
	// BucketTagging 是否支持 Bucket 标签
	BucketTagging bool
	// AdminAPI 是否提供 MinIO 管理面（用户 / 策略管理）
	AdminAPI bool
}

// Provider 是一类对象存储服务的差异点集合。
type Provider interface {
	// Key 配置中使用的厂商标识
	Key() string
	// RequiresExplicitRegion 表示签名区域必须由用户显式配置（公有云不按域名推导区域）。
	RequiresExplicitRegion() bool
	// BucketLookup 决定寻址方式（虚拟主机 / 路径）
	BucketLookup(style, endpointHost string) minio.BucketLookupType
	// AppID 从端点提取账号级后缀（腾讯云 COS 的 AppID），其他厂商返回空串
	AppID(endpoint string) string
	// NormalizeBucketName 按厂商命名规则归一化 Bucket 名
	NormalizeBucketName(appID, name string) (string, error)
	// Capabilities 控制面能力
	Capabilities() Capabilities
}

// providers 厂商注册表。
var providers = map[string]Provider{
	// 「其他 S3 兼容」：通用实现，控制面按标准 S3 全开
	"s3": genericProvider{adminAPI: true},
	"minio": minioProvider{
		genericProvider: genericProvider{adminAPI: true},
	},
	"aws": awsProvider{},
	"oss": ossProvider{},
	"cos": cosProvider{},
	"obs": obsProvider{},
}

// providerFor 按配置值取厂商实现；未填（空）按「其他 S3 兼容」处理，
// 未知取值回落通用 S3 兼容实现（不含管理面）。
func providerFor(name string) Provider {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		key = "s3"
	}
	if p, ok := providers[key]; ok {
		return p
	}
	return genericProvider{}
}

// providerKeys 按厂商标识排序返回注册表键，保证下发顺序可预期。
func providerKeys() []string {
	keys := make([]string, 0, len(providers))
	for key := range providers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// ProviderCapabilities 汇总各厂商的控制面能力，供 API 层下发给前端。
func ProviderCapabilities() []types.StorageProviderInfo {
	infos := make([]types.StorageProviderInfo, 0, len(providers))
	for _, key := range providerKeys() {
		caps := providers[key].Capabilities()
		infos = append(infos, types.StorageProviderInfo{
			Key: key,
			Capabilities: types.StorageCapabilities{
				BucketPolicy:  caps.BucketPolicy,
				SSEEncryption: caps.SSEEncryption,
				BucketTagging: caps.BucketTagging,
				AdminAPI:      caps.AdminAPI,
			},
		})
	}
	return infos
}

// normalizeHost 归一化主机名：去空格、去尾点、转小写。
func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
}

// storageDefaultRegion 是既推导不出区域、端点也不属于公有云时的兜底值（MinIO 不按区域分布数据）。
const storageDefaultRegion = "us-east-1"

// lookupByStyle 把配置里的「寻址方式」映射为 minio-go 的寻址模式。
// virtual-host 要求 bucket 是端点域名的子域名，IP / localhost 端点做不到，须强制路径方式；
// 留空或 auto 时，公有云走虚拟托管，自建存储交给 minio-go 探测。
func lookupByStyle(style, endpointHost string, publicCloud bool) minio.BucketLookupType {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "path", "path-style":
		return minio.BucketLookupPath
	case "virtual", "virtual-hosted", "virtual-hosted-style", "dns":
		if virtualHostCapable(endpointHost) {
			return minio.BucketLookupDNS
		}
		return minio.BucketLookupPath
	default:
		if publicCloud && virtualHostCapable(endpointHost) {
			return minio.BucketLookupDNS
		}
		return minio.BucketLookupAuto
	}
}

// virtualHostCapable 判断端点主机能否走 virtual-host 寻址：必须是真实域名，IP 与 localhost 都不行。
func virtualHostCapable(endpointHost string) bool {
	host := normalizeHost(endpointHost)
	if host == "" {
		return false
	}
	if net.ParseIP(host) != nil {
		return false
	}
	return host != "localhost" && !strings.HasSuffix(host, ".localhost")
}
