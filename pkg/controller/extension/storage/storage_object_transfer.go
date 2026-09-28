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
	"archive/zip"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// 对象跨桶复制/移动，以及批量（含目录）打包下载。
//
// 两者共用「把选中项展开成具体对象」的逻辑：目录在 S3 里只是前缀，
// 必须递归列举成具体 key 才能逐个处理。

const (
	// maxTransferObjects 单次复制/移动的对象数上限，防止一次请求把服务端拖住
	maxTransferObjects = 1000
	// maxArchiveObjects 单次打包下载的对象数上限
	maxArchiveObjects = 1000
	// maxArchiveObjectBytes 允许入包的单个对象大小上限
	maxArchiveObjectBytes = 5 << 30
	// maxCopyObjectBytes S3 CopyObject 单对象上限，超出需要 UploadPartCopy（暂不支持）
	maxCopyObjectBytes = 5 << 30
)

// transferItem 展开后的源/目标对象对
type transferItem struct {
	SrcKey string
	DstKey string
	Size   int64
}

func copyObjectTooLarge(size int64) bool {
	return size > maxCopyObjectBytes
}

// archiveEntry 待打包对象（Name 为 zip 内的相对路径，已做路径穿越清洗）
type archiveEntry struct {
	SrcKey   string
	Name     string
	Modified time.Time
}

// TransferObjects 在同一个对象存储实例内跨 Bucket 复制或移动对象。
//
// 走 S3 服务端 CopyObject，对象数据不经 Pixiu 转发，因此同实例内大文件也很快。
// 已知限制：S3 的 CopyObject 单对象上限 5GiB，超出需要 UploadPartCopy（暂不支持）。
func (c *controller) TransferObjects(ctx context.Context, datasourceID int64, in *types.StorageTransferRequest) (*types.StorageTransferResult, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, apierrors.NewError(fmt.Errorf("transfer config is required"), 400)
	}
	srcBucket, err := requireBucket(in.Bucket)
	if err != nil {
		return nil, err
	}
	dstBucket, err := requireBucket(in.DstBucket)
	if err != nil {
		return nil, err
	}
	if len(in.Keys) == 0 {
		return nil, apierrors.NewError(fmt.Errorf("object keys are required"), 400)
	}
	dstPrefix := normalizeObjectPrefix(in.DstPrefix)

	items, err := expandTransferItems(ctx, client, srcBucket, in.Keys, dstPrefix)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, apierrors.NewError(fmt.Errorf("no object matched the given selection"), 400)
	}
	if len(items) > maxTransferObjects {
		return nil, apierrors.NewError(
			fmt.Errorf("too many objects to transfer: %d (max %d)", len(items), maxTransferObjects), 400)
	}

	result := &types.StorageTransferResult{Total: len(items)}
	for _, item := range items {
		// 同桶同 key 的「移动到原位置」直接跳过，否则 CopyObject 到自身会报错
		if srcBucket == dstBucket && item.SrcKey == item.DstKey {
			result.Total--
			continue
		}
		if copyObjectTooLarge(item.Size) {
			result.Failed++
			result.FailedItems = append(result.FailedItems,
				fmt.Sprintf("%s: exceeds the 5GiB CopyObject limit", item.SrcKey))
			continue
		}
		if _, err := client.CopyObject(ctx,
			minio.CopyDestOptions{Bucket: dstBucket, Object: item.DstKey},
			minio.CopySrcOptions{Bucket: srcBucket, Object: item.SrcKey},
		); err != nil {
			result.Failed++
			result.FailedItems = append(result.FailedItems, fmt.Sprintf("%s: %v", item.SrcKey, err))
			continue
		}
		if in.Move {
			if err := client.RemoveObject(ctx, srcBucket, item.SrcKey, minio.RemoveObjectOptions{}); err != nil {
				// 已复制但源未删除：明确提示，用户可手工清理，避免误以为整体成功
				result.Failed++
				result.FailedItems = append(result.FailedItems,
					fmt.Sprintf("%s: copied to %s but failed to remove source: %v", item.SrcKey, item.DstKey, err))
				continue
			}
		}
		result.Succeeded++
	}
	return result, nil
}

// DownloadObjectsZip 把选中对象（目录递归展开）打包为 zip 流式写入 w。
//
// 返回值 written 表示响应体是否已经开始写出：
//   - false 时 handler 仍可正常返回 JSON 错误；
//   - true 时 HTTP header/body 已发出，只能中断连接，不能再改状态码。
//
// S3 没有「批量下载」接口，只能服务端逐个 GetObject 后打包。这里用 archive/zip
// 直接写 http.ResponseWriter（边读边写，不在内存里缓存整包），
// 并跳过写入失败的单个对象，保证个别对象异常不会毁掉整个压缩包。
func (c *controller) DownloadObjectsZip(ctx context.Context, datasourceID int64, bucket string, keys []string, w io.Writer) (bool, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return false, err
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return false, err
	}
	if len(keys) == 0 {
		return false, apierrors.NewError(fmt.Errorf("object keys are required"), 400)
	}
	entries, err := expandArchiveEntries(ctx, client, bucket, keys)
	if err != nil {
		return false, err
	}
	if len(entries) == 0 {
		return false, apierrors.NewError(fmt.Errorf("no object matched the given selection"), 400)
	}

	// 从这里开始才会写响应体
	zw := zip.NewWriter(w)
	defer zw.Close()
	for _, entry := range entries {
		if ctx.Err() != nil {
			return true, ctx.Err()
		}
		object, getErr := client.GetObject(ctx, bucket, entry.SrcKey, minio.GetObjectOptions{})
		if getErr != nil {
			continue
		}
		header := &zip.FileHeader{Name: entry.Name, Method: zip.Deflate}
		if !entry.Modified.IsZero() {
			header.Modified = entry.Modified
		}
		writer, createErr := zw.CreateHeader(header)
		if createErr != nil {
			object.Close()
			continue
		}
		if _, copyErr := io.Copy(writer, object); copyErr != nil {
			object.Close()
			// 多为客户端断开或对象读取失败：中断整包，写出的内容已不可回退
			return true, fmt.Errorf("write archive entry %s failed: %w", entry.Name, copyErr)
		}
		object.Close()
	}
	return true, nil
}

// expandTransferItems 把选中项展开成复制映射。
// 目录保留内部层级与目录名：选中 a/b/ 移动到 prefix 下即 prefix/b/...；
// 单对象取其文件名拼到目标前缀之后（与控制台「复制到」的直觉一致）。
func expandTransferItems(ctx context.Context, client *minio.Client, bucket string, keys []string, dstPrefix string) ([]transferItem, error) {
	items := make([]transferItem, 0, len(keys))
	seen := make(map[string]struct{})
	for _, raw := range keys {
		key := strings.TrimSpace(raw)
		if key == "" {
			continue
		}
		if !strings.HasSuffix(key, "/") {
			dst := dstPrefix + baseObjectName(key)
			if _, ok := seen[dst]; ok {
				continue
			}
			seen[dst] = struct{}{}
			var size int64
			if info, statErr := client.StatObject(ctx, bucket, key, minio.StatObjectOptions{}); statErr == nil {
				size = info.Size
			}
			items = append(items, transferItem{SrcKey: key, DstKey: dst, Size: size})
			continue
		}

		dirName := baseObjectName(key)
		prefix := dstPrefix + dirName + "/"
		expanded := 0
		for obj := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: key, Recursive: true}) {
			if obj.Err != nil {
				return nil, apierrors.NewError(fmt.Errorf("list storage objects failed: %w", obj.Err), 502)
			}
			dst := prefix + strings.TrimPrefix(obj.Key, key)
			if _, ok := seen[dst]; ok {
				continue
			}
			seen[dst] = struct{}{}
			items = append(items, transferItem{SrcKey: obj.Key, DstKey: dst, Size: obj.Size})
			expanded++
		}
		if expanded == 0 {
			// 空目录（只有占位对象）也要在目标侧保留，否则移动后目录消失
			dst := strings.TrimSuffix(prefix, "/") + "/"
			if _, ok := seen[dst]; !ok {
				seen[dst] = struct{}{}
				var size int64
				if info, statErr := client.StatObject(ctx, bucket, key, minio.StatObjectOptions{}); statErr == nil {
					size = info.Size
				}
				items = append(items, transferItem{SrcKey: key, DstKey: dst, Size: size})
			}
		}
	}
	return items, nil
}

// expandArchiveEntries 展开待打包对象，Name 为 zip 内相对路径。
// 目录占位对象不入包：zip 的目录结构由条目路径隐式创建。
func expandArchiveEntries(ctx context.Context, client *minio.Client, bucket string, keys []string) ([]archiveEntry, error) {
	entries := make([]archiveEntry, 0, len(keys))
	seen := make(map[string]struct{})
	for _, raw := range keys {
		key := strings.TrimSpace(raw)
		if key == "" {
			continue
		}
		if !strings.HasSuffix(key, "/") {
			info, err := client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
			if err != nil {
				// 单个对象取不到元数据（可能刚被删除）时跳过，不影响其余对象打包
				continue
			}
			if info.Size > maxArchiveObjectBytes {
				continue
			}
			name := sanitizeArchiveName(baseObjectName(key))
			if _, ok := seen[name]; ok {
				// 同名对象来自不同目录：退回完整 key，避免互相覆盖
				name = sanitizeArchiveName(key)
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			entries = append(entries, archiveEntry{SrcKey: key, Name: name, Modified: info.LastModified})
			if len(entries) >= maxArchiveObjects {
				return entries, nil
			}
			continue
		}

		dirName := baseObjectName(key)
		for obj := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: key, Recursive: true}) {
			if obj.Err != nil {
				return nil, apierrors.NewError(fmt.Errorf("list storage objects failed: %w", obj.Err), 502)
			}
			if strings.HasSuffix(obj.Key, "/") || obj.Size > maxArchiveObjectBytes {
				continue
			}
			name := sanitizeArchiveName(dirName + "/" + strings.TrimPrefix(obj.Key, key))
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			entries = append(entries, archiveEntry{SrcKey: obj.Key, Name: name, Modified: obj.LastModified})
			if len(entries) >= maxArchiveObjects {
				return entries, nil
			}
		}
	}
	return entries, nil
}

// normalizeObjectPrefix 归一化目标前缀：去掉首尾多余斜杠，非空时保证以单个 / 结尾
func normalizeObjectPrefix(prefix string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return ""
	}
	return prefix + "/"
}

// baseObjectName 取对象 key 的最后一段（目录 key 的结尾 / 会被忽略）
func baseObjectName(key string) string {
	segments := strings.Split(strings.TrimSuffix(strings.TrimSpace(key), "/"), "/")
	for i := len(segments) - 1; i >= 0; i-- {
		if segments[i] != "" {
			return segments[i]
		}
	}
	return strings.TrimSpace(key)
}

// sanitizeArchiveName 清洗 zip 条目名：去掉绝对路径与 . / .. 段，避免解压时路径穿越
func sanitizeArchiveName(name string) string {
	segments := make([]string, 0, 8)
	for _, seg := range strings.Split(strings.ReplaceAll(name, "\\", "/"), "/") {
		if seg == "" || seg == "." || seg == ".." {
			continue
		}
		segments = append(segments, seg)
	}
	if len(segments) == 0 {
		return "object"
	}
	return strings.Join(segments, "/")
}
