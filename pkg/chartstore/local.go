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

package chartstore

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// localStore 将 chart 包落到本地磁盘目录。
type localStore struct {
	dir string
}

// ensureLocalDir 确保本地存储目录存在并返回其绝对路径（供 newLocal 与启动预热共用）。
func ensureLocalDir(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("appstore local storage dir is empty")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(abs, 0o755); err != nil {
		return "", fmt.Errorf("failed to create chart dir %s: %w", abs, err)
	}
	return abs, nil
}

func newLocal(dir string) (Store, error) {
	abs, err := ensureLocalDir(dir)
	if err != nil {
		return nil, err
	}
	return &localStore{dir: abs}, nil
}

// resolve 将 key 收敛到存储目录内，防止路径穿越（../ 或绝对路径越界）。
func (s *localStore) resolve(key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("empty chart key")
	}
	if filepath.IsAbs(key) {
		return "", fmt.Errorf("chart key must be relative: %s", key)
	}
	cleaned := filepath.Clean(key)
	if cleaned == "." || strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid chart key: %s", key)
	}
	full := filepath.Join(s.dir, cleaned)
	// 二次确认最终路径仍位于存储目录内，杜绝穿越
	if full != s.dir && !strings.HasPrefix(full, s.dir+string(filepath.Separator)) {
		return "", fmt.Errorf("chart key escapes storage dir: %s", key)
	}
	return full, nil
}

func (s *localStore) Save(_ context.Context, key string, r io.Reader) error {
	full, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	tmp := full + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err = io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err = f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, full)
}

func (s *localStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	full, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

func (s *localStore) Delete(_ context.Context, key string) error {
	full, err := s.resolve(key)
	if err != nil {
		return err
	}
	err = os.Remove(full)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// LocalPath 实现 localPathProvider，本地后端可直接返回磁盘路径。
func (s *localStore) LocalPath(key string) (string, bool) {
	full, err := s.resolve(key)
	if err != nil {
		return "", false
	}
	return full, true
}
