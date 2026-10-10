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

package util

import "regexp"

// hostnameRe Linux 主机名格式（RFC 1123）：1-63 位，仅小写字母/数字/中划线，
// 且不能以中划线开头或结尾。与前端主机页校验保持一致。
var hostnameRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// IsValidHostname 校验主机名是否符合 Linux 规范（调用方需先 trim，与前端口径一致）。
// 节点写库与部署渲染前置校验共用，防畸形主机名注入 Ansible inventory 或路径穿越。
func IsValidHostname(name string) bool {
	return hostnameRe.MatchString(name)
}
