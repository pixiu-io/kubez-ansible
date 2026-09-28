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

	"github.com/minio/minio-go/v7"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// 对象存储多版本与回收站能力：历史版本列举、版本恢复与彻底删除、Bucket 版本控制状态。
// 全部基于 S3 数据面标准 API（不依赖 madmin），因此 MinIO / 阿里云 OSS / 腾讯云 COS / 华为云 OBS 通用。

const (
	// maxObjectVersions 历史版本单次展开上限。
	// ListObjectVersions 无法从指定版本续读（minio-go 未暴露 version-id-marker，StartAfter 在版本列举路径不生效），
	// 因此这里一次性展开到上限，超出仅标记 Truncated，不做游标分页，避免同 key 多版本在分页边界重复。
	maxObjectVersions = 2000
)

// ListObjectVersions 列举对象历史版本。
// onlyDeleteMarkers 为 true 时只返回删除标记（回收站视图），用于找回被删除的对象。
func (c *controller) ListObjectVersions(ctx context.Context, datasourceID int64, bucket, prefix string, onlyDeleteMarkers bool, limit int) (*types.StorageObjectVersionList, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > maxObjectVersions {
		limit = maxObjectVersions
	}

	items := make([]types.StorageObjectVersion, 0, 64)
	truncated := false
	for obj := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:       prefix,
		Recursive:    true,
		WithVersions: true,
		MaxKeys:      1000,
	}) {
		if obj.Err != nil {
			return nil, apierrors.NewError(fmt.Errorf("list storage object versions failed: %w", obj.Err), 502)
		}
		if onlyDeleteMarkers && !obj.IsDeleteMarker {
			continue
		}
		// S3 前缀匹配是「字符串前缀」而非「路径前缀」，
		// 前缀 "a/b.txt" 会连带匹配 "a/b.txt2"，因此这里做一次边界收紧。
		if !matchVersionPrefix(obj.Key, prefix) {
			continue
		}
		if len(items) >= limit {
			truncated = true
			break
		}
		lastModified := obj.LastModified
		items = append(items, types.StorageObjectVersion{
			Key:            obj.Key,
			VersionID:      obj.VersionID,
			IsLatest:       obj.IsLatest,
			IsDeleteMarker: obj.IsDeleteMarker,
			Size:           obj.Size,
			LastModified:   &lastModified,
			ETag:           obj.ETag,
			StorageClass:   obj.StorageClass,
		})
	}

	// 回收站跨多个对象，按删除时间倒序更方便先看到最近误删的对象
	if onlyDeleteMarkers {
		sort.SliceStable(items, func(i, j int) bool {
			return versionTimeAfter(items[i], items[j])
		})
	}

	state := bucketVersioningState(ctx, client, bucket)
	return &types.StorageObjectVersionList{
		Items:             items,
		Truncated:         truncated,
		VersioningState:   state,
		VersioningEnabled: state == versioningStateEnabled,
	}, nil
}

// 版本控制三态：unknown 用来区分「查询失败」与「确实未开启」。
// 查询失败若降级成「未开启」，MinIO 抖动时前端会给出错误结论（例如误判删除不可恢复）。
const (
	versioningStateEnabled  = "enabled"
	versioningStateDisabled = "disabled"
	versioningStateUnknown  = "unknown"
)

// bucketVersioningState 查询版本控制状态，查询失败返回 unknown（不阻断主流程）。
func bucketVersioningState(ctx context.Context, client *minio.Client, bucket string) string {
	cfg, err := client.GetBucketVersioning(ctx, bucket)
	if err != nil {
		return versioningStateUnknown
	}
	if cfg.Status == "Enabled" {
		return versioningStateEnabled
	}
	return versioningStateDisabled
}

// GetBucketVersioningState 查询 Bucket 版本控制状态（enabled/disabled/unknown），
// 供删除确认、回收站等场景使用；unknown 时前端应提示状态获取失败而不是断言未开启。
func (c *controller) GetBucketVersioningState(ctx context.Context, datasourceID int64, bucket string) (string, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return versioningStateUnknown, err
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return versioningStateUnknown, err
	}
	return bucketVersioningState(ctx, client, bucket), nil
}

// versionTimeAfter 按修改时间倒序比较（LastModified 由列举结果填充，恒非空）
func versionTimeAfter(a, b types.StorageObjectVersion) bool {
	return a.LastModified.After(*b.LastModified)
}

// matchVersionPrefix 按「路径前缀」匹配对象 key：
// 等于前缀本身，或位于前缀目录之下；避免 "a/b.txt" 误匹配到 "a/b.txt2"。
func matchVersionPrefix(key, prefix string) bool {
	if prefix == "" {
		return true
	}
	if key == prefix {
		return true
	}
	return strings.HasPrefix(key, strings.TrimSuffix(prefix, "/")+"/")
}

// findObjectVersion 在指定对象的历史版本中定位目标版本。
// 列举前缀精确到对象 key，因此即便对象版本很多，开销也可控。
func findObjectVersion(ctx context.Context, client *minio.Client, bucket, key, versionID string) (minio.ObjectInfo, bool) {
	for obj := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:       key,
		Recursive:    true,
		WithVersions: true,
		MaxKeys:      1000,
	}) {
		if obj.Err != nil {
			return minio.ObjectInfo{}, false
		}
		if obj.Key == key && obj.VersionID == versionID {
			return obj, true
		}
	}
	return minio.ObjectInfo{}, false
}

// RestoreObjectVersion 恢复历史版本。
// 目标版本是删除标记时，移除该标记即可让上一个版本重新成为当前版本；
// 否则把历史版本复制为新的当前版本（历史版本本身保留）。
func (c *controller) RestoreObjectVersion(ctx context.Context, datasourceID int64, bucket, key, versionID string) error {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return err
	}
	key, err = requireObjectKey(key)
	if err != nil {
		return err
	}
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return apierrors.NewError(fmt.Errorf("version id is required"), 400)
	}

	target, ok := findObjectVersion(ctx, client, bucket, key, versionID)
	if !ok {
		return apierrors.NewError(fmt.Errorf("object version %s not found", versionID), 404)
	}

	if target.IsDeleteMarker {
		if err := client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{VersionID: versionID}); err != nil {
			return apierrors.NewError(fmt.Errorf("remove delete marker failed: %w", err), 502)
		}
		return nil
	}

	if _, err := client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: bucket, Object: key},
		minio.CopySrcOptions{Bucket: bucket, Object: key, VersionID: versionID},
	); err != nil {
		return apierrors.NewError(fmt.Errorf("restore object version failed: %w", err), 502)
	}
	return nil
}

// DeleteObjectVersion 彻底删除指定历史版本（不可恢复，释放存储空间）
func (c *controller) DeleteObjectVersion(ctx context.Context, datasourceID int64, bucket, key, versionID string) error {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return err
	}
	key, err = requireObjectKey(key)
	if err != nil {
		return err
	}
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return apierrors.NewError(fmt.Errorf("version id is required"), 400)
	}

	if err := client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{VersionID: versionID}); err != nil {
		return apierrors.NewError(fmt.Errorf("delete object version failed: %w", err), 502)
	}
	return nil
}
