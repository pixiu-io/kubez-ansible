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

// MinIO（含自建 / 私有化部署）：唯一提供完整管理面（用户 / 策略）的厂商。
// 端点不做厂商假设，寻址方式交给 minio-go 探测。
type minioProvider struct {
	genericProvider
}

func (minioProvider) Key() string { return "minio" }

func (p minioProvider) Capabilities() Capabilities {
	caps := p.genericProvider.Capabilities()
	caps.AdminAPI = true
	return caps
}
