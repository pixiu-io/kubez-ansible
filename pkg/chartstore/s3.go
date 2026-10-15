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

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
)

// s3Store 基于 S3 兼容对象存储（MinIO / 阿里云 OSS / 华为云 OBS 等）的 chart 包存储。
type s3Store struct {
	client *minio.Client
	bucket string
	prefix string
}

func newS3(cfg config.S3Options) (Store, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("appstore s3 endpoint/bucket must not be empty")
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create s3 client: %w", err)
	}

	store := &s3Store{
		client: client,
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
	}

	// 惰性确保 bucket 存在：构造时不发网络请求（New 可能被每次请求调用）。
	ctx := context.Background()
	if exists, err := client.BucketExists(ctx, cfg.Bucket); err != nil {
		return nil, fmt.Errorf("failed to check s3 bucket %s: %w", cfg.Bucket, err)
	} else if !exists {
		if err = client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			return nil, fmt.Errorf("failed to create s3 bucket %s: %w", cfg.Bucket, err)
		}
	}

	return store, nil
}

func (s *s3Store) objectKey(key string) string {
	if s.prefix == "" {
		return key
	}
	return fmt.Sprintf("%s/%s", s.prefix, key)
}

func (s *s3Store) Save(ctx context.Context, key string, r io.Reader) error {
	_, err := s.client.PutObject(ctx, s.bucket, s.objectKey(key), r, -1, minio.PutObjectOptions{
		ContentType: "application/gzip",
	})
	if err != nil {
		return fmt.Errorf("failed to upload chart to s3: %w", err)
	}
	return nil
}

func (s *s3Store) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, s.objectKey(key), minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get chart from s3: %w", err)
	}
	// minio 的 GetObject 为惰性读取，用 Stat 提前暴露不存在等错误
	if _, err = obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, fmt.Errorf("chart not found in s3: %w", err)
	}
	return obj, nil
}

func (s *s3Store) Delete(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, s.objectKey(key), minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete chart from s3: %w", err)
	}
	return nil
}
