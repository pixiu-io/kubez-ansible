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
	"io"
	"sort"
	"strings"

	"github.com/minio/minio-go/v7"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// 大文件分片上传（S3 Multipart Upload）。
//
// 为什么单独做一条链路：单次 PUT 的对象大小受反向代理限制（例如 nginx 的
// client_max_body_size 默认只有 1MiB，部署环境常见 10MiB），大文件必然 413。
// 分片上传把文件切成多个小片各自 PUT，每个请求体都很小，天然绕过代理限制，
// 并且支持「单片失败只重传该片」与断点续传。
//
// minio-go v7.0.95 的 Core 已导出全部所需方法，无需自定义 HTTP 签名：
// NewMultipartUpload / PutObjectPart / ListObjectParts / CompleteMultipartUpload / AbortMultipartUpload。

const (
	// multipartPartSize 服务端下发的分片大小（8MiB）。
	// S3 规定除最后一片外每片不得小于 5MiB；8MiB 在请求数与单片耗时之间比较均衡。
	multipartPartSize = 8 << 20
	// multipartMaxParts S3 硬限制：单次分片上传最多 10000 片
	multipartMaxParts = 10000
	// multipartMaxPartBytes 单片字节上限，避免把整个文件塞进一个「分片」退化成普通 PUT，
	// 那样又会撞上反向代理的请求体限制
	multipartMaxPartBytes = 128 << 20
	// multipartListPartsLimit 列举已上传分片的单页上限
	multipartListPartsLimit = 1000
)

// InitMultipartUpload 初始化分片上传，返回 uploadId 与服务端建议的分片大小。
func (c *controller) InitMultipartUpload(ctx context.Context, datasourceID int64, in *types.StorageMultipartInit) (*types.StorageMultipartSession, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, apierrors.NewError(fmt.Errorf("multipart upload config is required"), 400)
	}
	bucket, err := requireBucket(in.Bucket)
	if err != nil {
		return nil, err
	}
	key, err := requireObjectKey(in.Key)
	if err != nil {
		return nil, err
	}
	core := minio.Core{Client: client}
	uploadID, err := core.NewMultipartUpload(ctx, bucket, key, minio.PutObjectOptions{
		ContentType:  strings.TrimSpace(in.ContentType),
		StorageClass: strings.TrimSpace(in.StorageClass),
	})
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("initiate multipart upload failed: %w", err), 502)
	}
	return &types.StorageMultipartSession{
		UploadID: uploadID,
		PartSize: multipartPartSize,
		MaxParts: multipartMaxParts,
	}, nil
}

// UploadMultipartPart 上传单个分片。分片序号从 1 开始，同一序号重复上传会覆盖前一片。
func (c *controller) UploadMultipartPart(ctx context.Context, datasourceID int64, bucket, key, uploadID string, partNumber int, reader io.Reader, size int64) (*types.StorageMultipartPart, error) {
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
	uploadID, err = requireUploadID(uploadID)
	if err != nil {
		return nil, err
	}
	if partNumber < 1 || partNumber > multipartMaxParts {
		return nil, apierrors.NewError(fmt.Errorf("part number must be between 1 and %d", multipartMaxParts), 400)
	}
	// minio-go 需要确切的字节数（请求体不是 io.ReadSeeker，无法自行探测），
	// 浏览器上传 Blob 时会带上 Content-Length，缺失说明客户端实现异常
	if size <= 0 {
		return nil, apierrors.NewError(fmt.Errorf("part content length is required"), 400)
	}
	if size > multipartMaxPartBytes {
		return nil, apierrors.NewError(
			fmt.Errorf("part size %d exceeds the limit of %d bytes", size, multipartMaxPartBytes), 400)
	}

	core := minio.Core{Client: client}
	part, err := core.PutObjectPart(ctx, bucket, key, uploadID, partNumber, reader, size, minio.PutObjectPartOptions{})
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("upload multipart part(%d) failed: %w", partNumber, err), 502)
	}
	return toMultipartPart(part), nil
}

// ListMultipartParts 列举某次分片上传已成功上传的分片，用于断点续传（只补传缺失的片）。
func (c *controller) ListMultipartParts(ctx context.Context, datasourceID int64, bucket, key, uploadID string) ([]types.StorageMultipartPart, error) {
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
	uploadID, err = requireUploadID(uploadID)
	if err != nil {
		return nil, err
	}

	core := minio.Core{Client: client}
	parts := make([]types.StorageMultipartPart, 0, 32)
	marker := 0
	for {
		result, err := core.ListObjectParts(ctx, bucket, key, uploadID, marker, multipartListPartsLimit)
		if err != nil {
			return nil, apierrors.NewError(fmt.Errorf("list multipart parts failed: %w", err), 502)
		}
		for _, part := range result.ObjectParts {
			parts = append(parts, *toMultipartPart(part))
		}
		if !result.IsTruncated {
			break
		}
		if result.NextPartNumberMarker <= marker {
			break
		}
		marker = result.NextPartNumberMarker
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })
	return parts, nil
}

// CompleteMultipartUpload 合并分片完成上传。分片清单必须来自真实上传成功的分片，
// 否则 S3 会以 EntityTooSmall / InvalidPart 拒绝。
func (c *controller) CompleteMultipartUpload(ctx context.Context, datasourceID int64, in *types.StorageMultipartComplete) (*types.StorageMultipartResult, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, apierrors.NewError(fmt.Errorf("multipart upload config is required"), 400)
	}
	bucket, err := requireBucket(in.Bucket)
	if err != nil {
		return nil, err
	}
	key, err := requireObjectKey(in.Key)
	if err != nil {
		return nil, err
	}
	uploadID, err := requireUploadID(in.UploadID)
	if err != nil {
		return nil, err
	}
	if len(in.Parts) == 0 {
		return nil, apierrors.NewError(fmt.Errorf("multipart parts are required"), 400)
	}

	parts := make([]minio.CompletePart, 0, len(in.Parts))
	for _, item := range in.Parts {
		etag := strings.TrimSpace(item.ETag)
		if item.PartNumber < 1 || etag == "" {
			return nil, apierrors.NewError(fmt.Errorf("part %d has no valid etag", item.PartNumber), 400)
		}
		parts = append(parts, minio.CompletePart{PartNumber: item.PartNumber, ETag: etag})
	}
	// S3 要求按分片序号升序提交，前端并发上传时提交顺序不保证
	sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })

	core := minio.Core{Client: client}
	info, err := core.CompleteMultipartUpload(ctx, bucket, key, uploadID, parts, minio.PutObjectOptions{})
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("complete multipart upload failed: %w", err), 502)
	}
	return &types.StorageMultipartResult{Key: info.Key, ETag: info.ETag, Size: info.Size}, nil
}

// AbortMultipartUpload 中止（取消）一次分片上传并释放已上传的分片。
func (c *controller) AbortMultipartUpload(ctx context.Context, datasourceID int64, in *types.StorageMultipartTarget) error {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	if in == nil {
		return apierrors.NewError(fmt.Errorf("multipart upload target is required"), 400)
	}
	bucket, err := requireBucket(in.Bucket)
	if err != nil {
		return err
	}
	key, err := requireObjectKey(in.Key)
	if err != nil {
		return err
	}
	uploadID, err := requireUploadID(in.UploadID)
	if err != nil {
		return err
	}
	core := minio.Core{Client: client}
	if err := core.AbortMultipartUpload(ctx, bucket, key, uploadID); err != nil {
		return apierrors.NewError(fmt.Errorf("abort multipart upload failed: %w", err), 502)
	}
	return nil
}

// requireUploadID 校验并裁剪 uploadId
func requireUploadID(uploadID string) (string, error) {
	uploadID = strings.TrimSpace(uploadID)
	if uploadID == "" {
		return "", apierrors.NewError(fmt.Errorf("upload id is required"), 400)
	}
	return uploadID, nil
}

func toMultipartPart(part minio.ObjectPart) *types.StorageMultipartPart {
	lastModified := part.LastModified
	return &types.StorageMultipartPart{
		PartNumber:   part.PartNumber,
		ETag:         part.ETag,
		Size:         part.Size,
		LastModified: &lastModified,
	}
}
