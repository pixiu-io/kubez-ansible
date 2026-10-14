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
	"github.com/minio/minio-go/v7/pkg/tags"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// 对象标签、对象详情与元数据更新：全部基于 S3 数据面标准 API（不依赖 madmin），多云通用。

// GetObjectTags 读取对象标签
func (c *controller) GetObjectTags(ctx context.Context, datasourceID int64, bucket, key, versionID string) ([]types.StorageObjectTag, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return nil, err
	}
	key, err = requireObjectKey(key)
	if err != nil {
		return nil, err
	}

	return getObjectTags(ctx, client, bucket, key, versionID)
}

func getObjectTags(ctx context.Context, client *minio.Client, bucket, key, versionID string) ([]types.StorageObjectTag, error) {
	tagSet, err := client.GetObjectTagging(ctx, bucket, key, minio.GetObjectTaggingOptions{VersionID: strings.TrimSpace(versionID)})
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("get object tags failed: %w", err), 502)
	}
	return toObjectTags(tagSet), nil
}

// SetObjectTags 覆盖写入对象标签；传入空列表表示清除全部标签
func (c *controller) SetObjectTags(ctx context.Context, datasourceID int64, bucket, key, versionID string, in []types.StorageObjectTag) error {
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

	tagMap := make(map[string]string, len(in))
	for _, item := range in {
		tagKey := strings.TrimSpace(item.Key)
		if tagKey == "" {
			continue
		}
		tagMap[tagKey] = strings.TrimSpace(item.Value)
	}

	if len(tagMap) == 0 {
		if err := client.RemoveObjectTagging(ctx, bucket, key, minio.RemoveObjectTaggingOptions{VersionID: versionID}); err != nil {
			return apierrors.NewError(fmt.Errorf("remove object tags failed: %w", err), 502)
		}
		return nil
	}

	tagSet, err := tags.NewTags(tagMap, true)
	if err != nil {
		return apierrors.NewError(fmt.Errorf("invalid object tags: %w", err), 400)
	}
	if err := client.PutObjectTagging(ctx, bucket, key, tagSet, minio.PutObjectTaggingOptions{VersionID: versionID}); err != nil {
		return apierrors.NewError(fmt.Errorf("set object tags failed: %w", err), 502)
	}
	return nil
}

func toObjectTags(tagSet *tags.Tags) []types.StorageObjectTag {
	if tagSet == nil {
		return []types.StorageObjectTag{}
	}
	out := make([]types.StorageObjectTag, 0, tagSet.Count())
	for tagKey, tagValue := range tagSet.ToMap() {
		out = append(out, types.StorageObjectTag{Key: tagKey, Value: tagValue})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// GetObjectDetail 读取对象详情与元数据：以 StatObject 为主，标签与 ACL 属增强信息，失败不影响主流程。
func (c *controller) GetObjectDetail(ctx context.Context, datasourceID int64, bucket, key, versionID string) (*types.StorageObjectDetail, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return nil, err
	}
	key, err = requireObjectKey(key)
	if err != nil {
		return nil, err
	}
	versionID = strings.TrimSpace(versionID)

	stat, err := client.StatObject(ctx, bucket, key, minio.StatObjectOptions{VersionID: versionID})
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("stat storage object failed: %w", err), 502)
	}

	lastModified := stat.LastModified
	detail := &types.StorageObjectDetail{
		Key:             stat.Key,
		VersionID:       stat.VersionID,
		Size:            stat.Size,
		ETag:            stat.ETag,
		ContentType:     stat.ContentType,
		ContentEncoding: stat.Metadata.Get("Content-Encoding"),
		CacheControl:    stat.Metadata.Get("Cache-Control"),
		LastModified:    &lastModified,
		StorageClass:    stat.StorageClass,
		UserMetadata:    make(map[string]string, len(stat.UserMetadata)),
	}
	for metaKey, metaValue := range stat.UserMetadata {
		detail.UserMetadata[metaKey] = metaValue
	}

	if tagList, tagErr := getObjectTags(ctx, client, bucket, key, versionID); tagErr == nil {
		detail.Tags = tagList
	}
	if aclInfo, aclErr := client.GetObjectACL(ctx, bucket, key); aclErr == nil && aclInfo != nil {
		detail.ACL = aclInfo.Metadata.Get("X-Amz-Acl")
	}

	return detail, nil
}

// UpdateObjectMeta 更新对象元数据。S3 没有独立的「改元数据」接口，只能 CopyObject + REPLACE 原地重写；
// REPLACE 会整体覆盖元数据，因此先读出当前值再合并，避免「只改 Content-Type」却把加密或存储类型丢掉。
func (c *controller) UpdateObjectMeta(ctx context.Context, datasourceID int64, bucket, key string, in *types.StorageObjectMetaUpdate) error {
	if in == nil {
		return apierrors.NewError(fmt.Errorf("object metadata is required"), 400)
	}
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

	stat, err := client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return apierrors.NewError(fmt.Errorf("stat storage object failed: %w", err), 502)
	}

	dest := minio.CopyDestOptions{
		Bucket:             bucket,
		Object:             key,
		ReplaceMetadata:    true,
		ContentType:        stat.ContentType,
		ContentEncoding:    stat.Metadata.Get("Content-Encoding"),
		ContentDisposition: stat.Metadata.Get("Content-Disposition"),
		ContentLanguage:    stat.Metadata.Get("Content-Language"),
		CacheControl:       stat.Metadata.Get("Cache-Control"),
		Expires:            stat.Expires,
		UserMetadata:       make(map[string]string, len(stat.UserMetadata)+4),
	}
	if sc := strings.TrimSpace(stat.StorageClass); sc != "" {
		dest.UserMetadata["X-Amz-Storage-Class"] = sc
	}
	if sseAlgo := stat.Metadata.Get("X-Amz-Server-Side-Encryption"); sseAlgo != "" {
		dest.UserMetadata["X-Amz-Server-Side-Encryption"] = sseAlgo
	}
	if kmsKey := stat.Metadata.Get("X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id"); kmsKey != "" {
		dest.UserMetadata["X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id"] = kmsKey
	}
	for metaKey, metaValue := range stat.UserMetadata {
		dest.UserMetadata[metaKey] = metaValue
	}

	if in.ContentType != nil {
		dest.ContentType = strings.TrimSpace(*in.ContentType)
	}
	if in.ContentEncoding != nil {
		dest.ContentEncoding = strings.TrimSpace(*in.ContentEncoding)
	}
	if in.CacheControl != nil {
		dest.CacheControl = strings.TrimSpace(*in.CacheControl)
	}
	if in.UserMetadata != nil {
		// Keep system metadata copied above; the editable user metadata replaces only prior user entries.
		for metaKey := range stat.UserMetadata {
			delete(dest.UserMetadata, metaKey)
		}
		for metaKey, metaValue := range *in.UserMetadata {
			metaKey = strings.TrimSpace(metaKey)
			if metaKey == "" {
				continue
			}
			dest.UserMetadata[metaKey] = metaValue
		}
	}

	if _, err := client.CopyObject(ctx, dest, minio.CopySrcOptions{Bucket: bucket, Object: key}); err != nil {
		return apierrors.NewError(fmt.Errorf("update object metadata failed: %w", err), 502)
	}
	return nil
}
