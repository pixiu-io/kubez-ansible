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

// Package chartstore 提供应用商店 chart 包的本地磁盘存储。
package chartstore

import (
	"context"
	"io"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
)

// Store chart 包存储后端。
// key 约定为 charts/<name>-<version>.tgz。
type Store interface {
	Save(ctx context.Context, key string, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// localPathProvider 由本地存储实现，用于在需要本地文件路径时（如 helm install）免去临时文件拷贝。
type localPathProvider interface {
	// LocalPath 返回 key 对应的本地绝对路径；非本地后端返回 ok=false。
	LocalPath(key string) (string, bool)
}

// New 构造本地存储后端。
func New(cfg config.AppstoreOptions) (Store, error) {
	return newLocal(cfg.DataDir)
}

// EnsureDataDir 启动阶段预热：确保 data_dir 存在并创建。
// 非致命——与「存储后端初始化失败不 panic，延迟到调用时报错」的设计保持一致。
func EnsureDataDir(cfg config.AppstoreOptions) error {
	_, err := ensureLocalDir(cfg.DataDir)
	return err
}

// AsLocalPath 若 store 为本地后端，返回 key 对应的本地路径。
func AsLocalPath(store Store, key string) (string, bool) {
	if p, ok := store.(localPathProvider); ok {
		return p.LocalPath(key)
	}
	return "", false
}
