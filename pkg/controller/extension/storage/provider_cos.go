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
	"fmt"
	"regexp"
	"strings"
)

// 腾讯云 COS。区域由用户显式填写；Bucket 名必须带所属账号的 AppID 后缀。
const cosProviderKey = "cos"

var (
	// cosBucketNameWithAppID Bucket 名是否已带 AppID 后缀
	cosBucketNameWithAppID = regexp.MustCompile(`-\d+$`)
)

type cosProvider struct {
	publicCloudProvider
}

func (cosProvider) Key() string { return cosProviderKey }

// NormalizeBucketName COS 的 Bucket 名必须带 AppID 后缀（由用户完整填写，不从 Endpoint 推导）。
func (cosProvider) NormalizeBucketName(appID, name string) (string, error) {
	if appID != "" {
		if strings.HasSuffix(name, "-"+appID) {
			return name, nil
		}
		return name + "-" + appID, nil
	}
	if cosBucketNameWithAppID.MatchString(name) {
		return name, nil
	}
	return "", fmt.Errorf("COS bucket name must include AppID in the format <bucketname>-<appid>")
}
