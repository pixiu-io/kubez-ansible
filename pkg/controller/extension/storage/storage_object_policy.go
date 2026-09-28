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
	"encoding/json"
	"fmt"
	"strings"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// Bucket Policy 原始策略读写。
//
// 与 Bucket 配置里的「私有 / 公共读 / 公共读写」三档归类（StorageBucketConfig.ACL）互补：
// 三档归类是为了「一眼看懂当前暴露面」，这里是完整的策略 JSON，支持精细授权，
// 例如只放开某前缀的匿名读取、强制要求加密上传、拒绝非指定来源的请求等，
// 语义与云厂商控制台的「Bucket 授权策略」一致。

// maxBucketPolicyBytes S3 对策略文档的限制是 20KB
const maxBucketPolicyBytes = 20 << 10

// GetBucketPolicy 读取 Bucket 策略原文。无策略时返回空的 Policy（Exists=false），
// 而不是报错——「没有策略」是正常状态。
func (c *controller) GetBucketPolicy(ctx context.Context, datasourceID int64, bucket string) (*types.StorageBucketPolicy, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return nil, err
	}
	policy, err := client.GetBucketPolicy(ctx, bucket)
	if err != nil {
		if hasErrorCode(err, "NoSuchBucketPolicy") {
			return &types.StorageBucketPolicy{ACL: "private"}, nil
		}
		return nil, apierrors.NewError(fmt.Errorf("get bucket policy failed: %w", err), 502)
	}
	return &types.StorageBucketPolicy{
		Policy: policy,
		Exists: strings.TrimSpace(policy) != "",
		ACL:    classifyBucketPolicy(policy),
	}, nil
}

// SetBucketPolicy 保存策略原文；空串表示删除策略（回到「仅授权用户可访问」）。
//
// 注意：这里改的是匿名访问策略，与 Bucket 配置里的「读写权限」是同一份底层数据，
// 两处修改会互相覆盖——前端在保存策略后需要提示这一点。
func (c *controller) SetBucketPolicy(ctx context.Context, datasourceID int64, bucket, policy string) error {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return err
	}
	policy = strings.TrimSpace(policy)
	if policy == "" {
		if err := client.SetBucketPolicy(ctx, bucket, ""); err != nil {
			return apierrors.NewError(fmt.Errorf("remove bucket policy failed: %w", err), 502)
		}
		return nil
	}
	if len(policy) > maxBucketPolicyBytes {
		return apierrors.NewError(
			fmt.Errorf("bucket policy is too large: %d bytes, limit is %d", len(policy), maxBucketPolicyBytes), 400)
	}
	if err := validateBucketPolicy(policy); err != nil {
		return err
	}
	if err := client.SetBucketPolicy(ctx, bucket, policy); err != nil {
		return apierrors.NewError(fmt.Errorf("set bucket policy failed: %w", err), 502)
	}
	return nil
}

// policyStatement 策略语句：只解析归类与校验需要的字段，其余内容原样透传给 S3。
// Sid / Condition 带 omitempty：buildBucketPolicy 复用该结构体生成策略原文，
// 空值若被序列化成 null 会被 S3 判为 MalformedPolicy。
type policyStatement struct {
	Sid       string      `json:"Sid,omitempty"`
	Effect    string      `json:"Effect"`
	Principal interface{} `json:"Principal"`
	Action    interface{} `json:"Action"`
	Resource  interface{} `json:"Resource"`
	Condition interface{} `json:"Condition,omitempty"`
}

type bucketPolicyDoc struct {
	Version   string            `json:"Version"`
	Statement []policyStatement `json:"Statement"`
}

// flattenPolicyValues 把策略字段的三种合法写法（字符串 / 字符串数组 / 对象）摊平成字符串列表，
// 便于统一做 Action、Resource、Principal 的判空与归类。
func flattenPolicyValues(value interface{}) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case map[string]interface{}:
		result := make([]string, 0)
		for _, item := range v {
			result = append(result, flattenPolicyValues(item)...)
		}
		return result
	}
	return nil
}

func containsAny(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// classifyBucketPolicy 将匿名访问策略归类为读写权限级别，供 Bucket 配置展示当前暴露面。
// 策略解析失败按最小权限处理（private），避免把异常策略误报成对外开放。
func classifyBucketPolicy(policyText string) string {
	if strings.TrimSpace(policyText) == "" {
		return "private"
	}
	var doc bucketPolicyDoc
	if err := json.Unmarshal([]byte(policyText), &doc); err != nil {
		return "private"
	}
	hasGet, hasWrite := false, false
	for _, stmt := range doc.Statement {
		if stmt.Effect != "Allow" {
			continue
		}
		// 只归类「匿名可访问」的策略，指定 Principal 的授权不算对外开放
		if !containsAny(flattenPolicyValues(stmt.Principal), "*") {
			continue
		}
		for _, action := range flattenPolicyValues(stmt.Action) {
			switch action {
			case "s3:*", "*":
				hasGet, hasWrite = true, true
			case "s3:GetObject":
				hasGet = true
			case "s3:PutObject", "s3:DeleteObject":
				hasWrite = true
			}
		}
	}
	switch {
	case hasGet && hasWrite:
		return "public-read-write"
	case hasGet:
		return "public-read"
	default:
		return "private"
	}
}

// buildBucketPolicy 生成匿名访问策略，public-read 只读，public-read-write 可读写
func buildBucketPolicy(bucket, acl string) string {
	actions := []string{"s3:GetObject"}
	if acl == "public-read-write" {
		actions = append(actions, "s3:PutObject", "s3:DeleteObject")
	}
	doc := bucketPolicyDoc{
		Version: "2012-10-17",
		Statement: []policyStatement{
			{
				Effect:    "Allow",
				Principal: "*",
				Action:    actions,
				Resource:  []string{fmt.Sprintf("arn:aws:s3:::%s/*", bucket)},
			},
		},
	}
	data, _ := json.Marshal(doc)
	return string(data)
}

// validateBucketPolicy 在提交给 S3 之前做本地结构校验。
// 服务端报错信息（MalformedPolicy）很难定位到具体语句，先在本地拦一遍能省一轮往返。
func validateBucketPolicy(text string) error {
	var doc bucketPolicyDoc
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		return apierrors.NewError(fmt.Errorf("bucket policy is not valid JSON: %w", err), 400)
	}
	if len(doc.Statement) == 0 {
		return apierrors.NewError(fmt.Errorf("bucket policy requires a non-empty Statement array"), 400)
	}
	for i, stmt := range doc.Statement {
		switch strings.ToLower(strings.TrimSpace(stmt.Effect)) {
		case "allow", "deny":
		default:
			return apierrors.NewError(fmt.Errorf("statement[%d]: Effect must be Allow or Deny", i), 400)
		}
		if stmt.Principal == nil {
			return apierrors.NewError(fmt.Errorf("statement[%d]: Principal is required", i), 400)
		}
		if len(flattenPolicyValues(stmt.Action)) == 0 {
			return apierrors.NewError(fmt.Errorf("statement[%d]: Action is required", i), 400)
		}
		if len(flattenPolicyValues(stmt.Resource)) == 0 {
			return apierrors.NewError(fmt.Errorf("statement[%d]: Resource is required", i), 400)
		}
	}
	return nil
}
