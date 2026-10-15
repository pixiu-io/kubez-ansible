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

// Package chartutil 提供 Helm Chart 包的解析与基础校验能力。
// 仅复用 helm.sh/helm/v3 的 loader（项目已依赖该库，不引入新的重型依赖）。
package chartutil

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"helm.sh/helm/v3/pkg/chart/loader"
)

// Metadata 从 chart 包中解析出的元信息。
type Metadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	AppVersion  string `json:"app_version"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	// Type chart 类型，application | library；应用商店仅接受 application
	Type string `json:"type"`

	// Sha256 chart 包的 sha256 摘要（十六进制小写）
	Sha256 string `json:"sha256"`
	// Size chart 包字节数
	Size int64 `json:"size"`
}

// Parse 解析 .tgz chart 包，返回元信息并计算 sha256/size。
// 使用 helm loader 解析，内部会做 name/semver 等合法性校验。
func Parse(data []byte) (*Metadata, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty chart archive")
	}

	ch, err := loader.LoadArchive(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid chart archive: %w", err)
	}
	if ch == nil || ch.Metadata == nil {
		return nil, fmt.Errorf("chart metadata not found")
	}

	sum := sha256.Sum256(data)
	return &Metadata{
		Name:        strings.TrimSpace(ch.Metadata.Name),
		Version:     strings.TrimSpace(ch.Metadata.Version),
		AppVersion:  strings.TrimSpace(ch.Metadata.AppVersion),
		Description: ch.Metadata.Description,
		Icon:        ch.Metadata.Icon,
		Type:        ch.Metadata.Type,
		Sha256:      hex.EncodeToString(sum[:]),
		Size:        int64(len(data)),
	}, nil
}

// ParseReader 读取全部内容后解析；调用方需自行限制读取大小（如 MaxUploadMB）。
func ParseReader(r io.Reader) (*Metadata, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// IsApplication 判断是否为可直接安装的 application 类型（空类型视为 application）。
func IsApplication(md *Metadata) bool {
	if md == nil {
		return false
	}
	return md.Type == "" || md.Type == "application"
}
