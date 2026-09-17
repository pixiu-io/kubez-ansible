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

// 对象存储对象级增强能力：多版本与回收站、碎片管理、对象标签、对象详情与元数据。
// 全部基于 S3 数据面标准 API（不依赖 madmin），因此 MinIO / 阿里云 OSS / 腾讯云 COS / 华为云 OBS 通用。

const (
	// maxObjectVersions 历史版本单次展开上限。
	// ListObjectVersions 无法从指定版本续读（minio-go 未暴露 version-id-marker，StartAfter 在版本列举路径不生效），
	// 因此这里一次性展开到上限，超出仅标记 Truncated，不做游标分页，避免同 key 多版本在分页边界重复。
	maxObjectVersions = 2000
	// maxIncompleteUploads 碎片单次扫描上限
	maxIncompleteUploads = 2000
)

// requireBucket 校验并裁剪 Bucket 名称
func requireBucket(bucket string) (string, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return "", apierrors.NewError(fmt.Errorf("bucket is required"), 400)
	}
	return bucket, nil
}

// requireObjectKey 校验并裁剪对象名称
func requireObjectKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", apierrors.NewError(fmt.Errorf("object key is required"), 400)
	}
	return key, nil
}

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

	return &types.StorageObjectVersionList{
		Items:     items,
		Truncated: truncated,
		// 查询版本控制状态失败时按未开启处理，让前端给出保守提示
		VersioningEnabled: bucketVersioningEnabled(ctx, client, bucket),
	}, nil
}

// bucketVersioningEnabled 查询 Bucket 是否开启了版本控制。
// 查询失败不阻断主流程，返回 false（前端按「未开启」给出提示）。
func bucketVersioningEnabled(ctx context.Context, client *minio.Client, bucket string) bool {
	cfg, err := client.GetBucketVersioning(ctx, bucket)
	if err != nil {
		return false
	}
	return cfg.Status == "Enabled"
}

// GetBucketVersioningEnabled 查询 Bucket 版本控制是否开启（供删除确认等场景使用）。
func (c *controller) GetBucketVersioningEnabled(ctx context.Context, datasourceID int64, bucket string) (bool, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return false, err
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return false, err
	}
	return bucketVersioningEnabled(ctx, client, bucket), nil
}

func versionTimeAfter(a, b types.StorageObjectVersion) bool {
	if a.LastModified == nil {
		return false
	}
	if b.LastModified == nil {
		return true
	}
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

// GetObjectDetail 读取对象详情与元数据。
// 以 StatObject 为主（跨厂商通用），标签与对象 ACL 属增强信息，失败不影响主流程。
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

	if tagList, tagErr := c.GetObjectTags(ctx, datasourceID, bucket, key, versionID); tagErr == nil {
		detail.Tags = tagList
	}
	if aclInfo, aclErr := client.GetObjectACL(ctx, bucket, key); aclErr == nil && aclInfo != nil {
		detail.ACL = aclInfo.Metadata.Get("X-Amz-Acl")
	}

	return detail, nil
}

// UpdateObjectMeta 更新对象元数据。
// S3 没有独立的「改元数据」接口，只能通过 CopyObject + x-amz-metadata-directive: REPLACE 原地重写；
// REPLACE 会整体覆盖元数据，因此这里先读出当前值再合并，避免「只改 Content-Type」却清空自定义元数据。
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
		Bucket:          bucket,
		Object:          key,
		ReplaceMetadata: true,
		ContentType:     stat.ContentType,
		ContentEncoding: stat.Metadata.Get("Content-Encoding"),
		CacheControl:    stat.Metadata.Get("Cache-Control"),
		UserMetadata:    make(map[string]string, len(stat.UserMetadata)),
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
		dest.UserMetadata = make(map[string]string, len(*in.UserMetadata))
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
