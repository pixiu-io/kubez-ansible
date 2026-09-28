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
	"context"
	"fmt"
	"sort"
	"strings"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// 对象存储碎片管理：列举与清理未完成的分片上传。

const (
	// maxIncompleteUploads 碎片单次扫描上限
	maxIncompleteUploads = 2000
)

// ListIncompleteUploads 列举未完成的分片上传（碎片）。
// 碎片不属于任何对象也不出现在对象列表里，但占用存储空间，是磁盘水位虚高的常见原因。
func (c *controller) ListIncompleteUploads(ctx context.Context, datasourceID int64, bucket, prefix string) (*types.StorageMultipartUploadList, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return nil, err
	}

	listCtx, cancelList := context.WithCancel(ctx)
	defer cancelList()

	items := make([]types.StorageMultipartUpload, 0, 32)
	var totalSize int64
	truncated := false
	for info := range client.ListIncompleteUploads(listCtx, bucket, prefix, true) {
		if truncated {
			continue
		}
		if info.Err != nil {
			return nil, apierrors.NewError(fmt.Errorf("list incomplete uploads failed: %w", info.Err), 502)
		}
		if len(items) >= maxIncompleteUploads {
			truncated = true
			cancelList()
			continue
		}
		initiated := info.Initiated
		items = append(items, types.StorageMultipartUpload{
			Key:       info.Key,
			UploadID:  info.UploadID,
			Initiated: &initiated,
			Size:      info.Size,
		})
		totalSize += info.Size
	}

	// 按发起时间升序，先清理最老的碎片（Initiated 由列举结果填充，恒非空）
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Initiated.Before(*items[j].Initiated)
	})

	return &types.StorageMultipartUploadList{Items: items, TotalSize: totalSize, Truncated: truncated}, nil
}

// AbortIncompleteUploads 清理指定对象（key）的全部未完成分片上传，并返回结果明细。
//
// 粒度：这里是「按对象」清理——同一 key 下的多条未完成上传会一起被中止。
// 需要精确中止某一次上传时用 AbortMultipartUpload（Core.AbortMultipartUpload 已导出）。
//
// 逐条失败只累计到 FailedKeys 而不整体返回 502：前端需要区分「哪些清理成功」，
// 只有参数与连接类错误才走 error。
func (c *controller) AbortIncompleteUploads(ctx context.Context, datasourceID int64, bucket string, keys []string) (*types.StorageAbortUploadResult, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, apierrors.NewError(fmt.Errorf("object keys are required"), 400)
	}

	result := &types.StorageAbortUploadResult{}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if err := client.RemoveIncompleteUpload(ctx, bucket, key); err != nil {
			result.Failed++
			result.FailedKeys = append(result.FailedKeys, key)
			continue
		}
		result.Succeeded++
	}
	return result, nil
}
