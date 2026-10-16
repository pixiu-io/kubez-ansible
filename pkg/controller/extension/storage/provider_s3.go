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
	"github.com/minio/minio-go/v7"
)

// genericProvider 是「其他 S3 兼容 / 未知厂商」的实现，同时作为各厂商实现的默认骨架：
// 端点原样使用、区域未填时回落默认值、寻址交给 minio-go 探测、控制面按标准 S3 全开。
type genericProvider struct{}

func (genericProvider) Key() string { return "s3" }

func (genericProvider) RequiresExplicitRegion() bool { return false }

func (genericProvider) BucketLookup(style, endpointHost string) minio.BucketLookupType {
	return lookupByStyle(style, endpointHost, false)
}

func (genericProvider) AppID(string) string { return "" }

func (genericProvider) NormalizeBucketName(_, name string) (string, error) { return name, nil }

func (genericProvider) Capabilities() Capabilities {
	return Capabilities{
		BucketPolicy:  true,
		SSEEncryption: true,
		BucketTagging: true,
	}
}

// publicCloudProvider 是公有云实现的公共部分，各厂商只需实现 Key 与自己的命名 / 能力例外。
type publicCloudProvider struct {
	genericProvider
}

func (publicCloudProvider) RequiresExplicitRegion() bool { return true }

// BucketLookup 公有云一律走虚拟主机寻址：OSS 只接受三级域名，COS、OBS 已禁止 path 寻址，
// 而 minio-go 的 Auto 只对 AWS 与阿里云开虚拟托管，COS、OBS 会退化成 path。
func (publicCloudProvider) BucketLookup(style, endpointHost string) minio.BucketLookupType {
	return lookupByStyle(style, endpointHost, true)
}

// Capabilities 公有云控制面收敛：授权与加密不兼容通用 S3 策略文档与 SSE-S3，改由控制台管理。
func (publicCloudProvider) Capabilities() Capabilities {
	return Capabilities{
		BucketPolicy:  false,
		SSEEncryption: false,
		BucketTagging: true,
	}
}
