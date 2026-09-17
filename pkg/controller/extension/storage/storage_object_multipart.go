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

	items := make([]types.StorageMultipartUpload, 0, 32)
	var totalSize int64
	truncated := false
	for info := range client.ListIncompleteUploads(ctx, bucket, prefix, true) {
		if info.Err != nil {
			return nil, apierrors.NewError(fmt.Errorf("list incomplete uploads failed: %w", info.Err), 502)
		}
		if len(items) >= maxIncompleteUploads {
			truncated = true
			break
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

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Initiated == nil {
			return false
		}
		if items[j].Initiated == nil {
			return true
		}
		return items[i].Initiated.Before(*items[j].Initiated)
	})

	return &types.StorageMultipartUploadList{Items: items, TotalSize: totalSize, Truncated: truncated}, nil
}

// AbortIncompleteUploads 清理指定对象的全部未完成分片上传。
// minio-go 只暴露「按对象清理」，不提供按 uploadId 单独中止，因此同一对象的多条碎片会一起被清理。
func (c *controller) AbortIncompleteUploads(ctx context.Context, datasourceID int64, bucket string, keys []string) error {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return apierrors.NewError(fmt.Errorf("object keys are required"), 400)
	}

	failed := make([]string, 0)
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if err := client.RemoveIncompleteUpload(ctx, bucket, key); err != nil {
			failed = append(failed, key)
		}
	}
	if len(failed) > 0 {
		return apierrors.NewError(
			fmt.Errorf("abort incomplete upload failed for %d object(s): %s", len(failed), strings.Join(failed, ", ")),
			502,
		)
	}
	return nil
}
