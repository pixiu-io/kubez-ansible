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

// 阿里云 OSS。端点需填写已启用 S3 兼容访问的服务级端点，区域由用户显式填写。
const ossProviderKey = "oss"

type ossProvider struct {
	publicCloudProvider
}

func (ossProvider) Key() string { return ossProviderKey }
