package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	madmin "github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/cors"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/minio/minio-go/v7/pkg/sse"
	"github.com/minio/minio-go/v7/pkg/tags"
	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type Interface interface {
	PingAdhoc(context.Context, *types.StorageSourceConfig) (*types.StoragePing, error)
	Ping(context.Context, int64) (*types.StoragePing, error)
	ListBuckets(context.Context, int64) ([]types.StorageBucket, error)
	CreateBucket(context.Context, int64, string) error
	DeleteBucket(context.Context, int64, string) error
	GetBucketConfig(context.Context, int64, string) (*types.StorageBucketConfig, error)
	SetBucketConfig(context.Context, int64, string, *types.StorageBucketConfigUpdate) error
	EmptyBucket(context.Context, int64, string) error
	GetLifecycle(context.Context, int64, string) ([]types.StorageLifecycleRule, error)
	SetLifecycle(context.Context, int64, string, []types.StorageLifecycleRule) error
	GetCors(context.Context, int64, string) ([]types.StorageCorsRule, error)
	SetCors(context.Context, int64, string, []types.StorageCorsRule) error
	ListObjects(context.Context, int64, string, string) ([]types.StorageObject, error)
	ListObjectsPage(context.Context, int64, string, string, string, int, string, bool) (*types.StorageObjectPage, error)
	PutObject(context.Context, int64, string, string, io.Reader, int64, string) error
	DeleteObjects(context.Context, int64, string, []string) error
	RenameObject(context.Context, int64, string, string, string) error
	ListObjectVersions(context.Context, int64, string, string, bool, int) (*types.StorageObjectVersionList, error)
	GetBucketVersioningEnabled(context.Context, int64, string) (bool, error)
	RestoreObjectVersion(context.Context, int64, string, string, string) error
	DeleteObjectVersion(context.Context, int64, string, string, string) error
	ListIncompleteUploads(context.Context, int64, string, string) (*types.StorageMultipartUploadList, error)
	AbortIncompleteUploads(context.Context, int64, string, []string) error
	GetObjectTags(context.Context, int64, string, string, string) ([]types.StorageObjectTag, error)
	SetObjectTags(context.Context, int64, string, string, string, []types.StorageObjectTag) error
	GetObjectDetail(context.Context, int64, string, string, string) (*types.StorageObjectDetail, error)
	UpdateObjectMeta(context.Context, int64, string, string, *types.StorageObjectMetaUpdate) error
	PresignedGetObject(context.Context, int64, string, string, int) (string, error)
	ListUsers(context.Context, int64) ([]types.StorageUser, error)
	CreateUser(context.Context, int64, *types.StorageUserCreate) error
	DeleteUser(context.Context, int64, string) error
	ListPolicies(context.Context, int64) ([]string, error)
	GetOverview(context.Context, int64) (*types.StorageOverview, error)
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

type clientConfig struct {
	provider string
	endpoint string
	region   string
	access   string
	secret   string
	token    string
	secure   bool
}

func (c *controller) clientFor(ctx context.Context, datasourceID int64) (*minio.Client, clientConfig, error) {
	object, err := c.factory.Datasource().Get(ctx, datasourceID)
	if err != nil {
		klog.Errorf("failed to get datasource(%d): %v", datasourceID, err)
		return nil, clientConfig{}, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, clientConfig{}, apierrors.NewError(fmt.Errorf("datasource not found"), 404)
	}
	if object.Type != model.DatasourceTypeMiddleware || object.SubType != model.DatasourceSubTypeStorage {
		return nil, clientConfig{}, apierrors.NewError(fmt.Errorf("datasource(%d) is not a storage datasource", datasourceID), 400)
	}
	if err := controllerutil.CheckResourceAccess(ctx, c.factory, object.UserId, types.ResourceTypeDatasource, datasourceID); err != nil {
		return nil, clientConfig{}, err
	}
	var cfg types.DatasourceConfig
	if err := cfg.Unmarshal(object.Config); err != nil {
		return nil, clientConfig{}, apierrors.ErrServerInternal
	}
	if cfg.Storage == nil {
		return nil, clientConfig{}, apierrors.NewError(fmt.Errorf("datasource(%d) missing storage config", datasourceID), 400)
	}
	sc := cfg.Storage
	endpoint := strings.TrimSpace(sc.Endpoint)
	if endpoint == "" && cfg.Log != nil {
		endpoint = strings.TrimSpace(cfg.Log.URL)
	}
	if endpoint == "" {
		return nil, clientConfig{}, apierrors.NewError(fmt.Errorf("storage endpoint is required"), 400)
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, clientConfig{}, apierrors.NewError(fmt.Errorf("invalid storage endpoint"), 400)
	}
	access, secret, token := sc.AccessKeyID, sc.SecretAccessKey, sc.SessionToken
	// Backward compatibility for credentials entered in the old generic auth fields.
	if access == "" && cfg.Log != nil {
		access, secret = cfg.Log.UserName, cfg.Log.Password
	}
	if access == "" || secret == "" {
		return nil, clientConfig{}, apierrors.NewError(fmt.Errorf("storage access key and secret key are required"), 400)
	}
	region := sc.Region
	if region == "" {
		region = "us-east-1"
	}
	mc, err := minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(access, secret, token),
		Secure: u.Scheme == "https",
		Region: region,
	})
	if err != nil {
		return nil, clientConfig{}, apierrors.NewError(fmt.Errorf("create storage client: %w", err), 400)
	}
	return mc, clientConfig{provider: sc.Provider, endpoint: endpoint, region: region, access: access, secret: secret, token: token, secure: u.Scheme == "https"}, nil
}

func (c *controller) Ping(ctx context.Context, datasourceID int64) (*types.StoragePing, error) {
	client, cc, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	_, err = client.ListBuckets(ctx)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("storage ping failed: %w", err), 502)
	}
	return &types.StoragePing{Connected: true, Provider: cc.provider}, nil
}

func (c *controller) PingAdhoc(ctx context.Context, sc *types.StorageSourceConfig) (*types.StoragePing, error) {
	client, provider, err := clientFromConfig(sc)
	if err != nil {
		return nil, err
	}
	if _, err = client.ListBuckets(ctx); err != nil {
		return nil, apierrors.NewError(fmt.Errorf("storage ping failed: %w", err), 502)
	}
	return &types.StoragePing{Connected: true, Provider: provider}, nil
}

func clientFromConfig(sc *types.StorageSourceConfig) (*minio.Client, string, error) {
	if sc == nil {
		return nil, "", apierrors.NewError(fmt.Errorf("storage config is required"), 400)
	}
	endpoint := strings.TrimSpace(sc.Endpoint)
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, "", apierrors.NewError(fmt.Errorf("invalid storage endpoint"), 400)
	}
	if strings.TrimSpace(sc.AccessKeyID) == "" || sc.SecretAccessKey == "" {
		return nil, "", apierrors.NewError(fmt.Errorf("storage access key and secret key are required"), 400)
	}
	region := strings.TrimSpace(sc.Region)
	if region == "" {
		region = "us-east-1"
	}
	client, err := minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(sc.AccessKeyID, sc.SecretAccessKey, sc.SessionToken),
		Secure: u.Scheme == "https", Region: region,
	})
	if err != nil {
		return nil, "", apierrors.NewError(fmt.Errorf("create storage client: %w", err), 400)
	}
	return client, sc.Provider, nil
}

func (c *controller) ListBuckets(ctx context.Context, datasourceID int64) ([]types.StorageBucket, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	buckets, err := client.ListBuckets(ctx)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("list storage buckets failed: %w", err), 502)
	}
	result := make([]types.StorageBucket, 0, len(buckets))
	for _, item := range buckets {
		result = append(result, types.StorageBucket{Name: item.Name, CreationDate: item.CreationDate})
	}
	return result, nil
}

func (c *controller) ListObjects(ctx context.Context, datasourceID int64, bucket, prefix string) ([]types.StorageObject, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, apierrors.NewError(fmt.Errorf("bucket is required"), 400)
	}
	result := make([]types.StorageObject, 0)
	for item := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if item.Err != nil {
			return nil, apierrors.NewError(fmt.Errorf("list storage objects failed: %w", item.Err), 502)
		}
		result = append(result, types.StorageObject{Key: item.Key, Size: item.Size, LastModified: &item.LastModified, ETag: item.ETag})
	}
	return result, nil
}

func (c *controller) PresignedGetObject(ctx context.Context, datasourceID int64, bucket, objectKey string, expirySeconds int) (string, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(bucket) == "" || strings.TrimSpace(objectKey) == "" {
		return "", apierrors.NewError(fmt.Errorf("bucket and object key are required"), 400)
	}
	if expirySeconds <= 0 || expirySeconds > 3600 {
		expirySeconds = 900
	}
	presigned, err := client.PresignedGetObject(ctx, bucket, objectKey, time.Duration(expirySeconds)*time.Second, nil)
	if err != nil {
		return "", apierrors.NewError(fmt.Errorf("create presigned url failed: %w", err), 502)
	}
	return presigned.String(), nil
}

// bucketNameRegex S3 Bucket 命名规范：3-63 位小写字母、数字、点、连字符，首尾为字母或数字
var bucketNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

func (c *controller) CreateBucket(ctx context.Context, datasourceID int64, name string) error {
	client, cc, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if !bucketNameRegex.MatchString(name) {
		return apierrors.NewError(fmt.Errorf("invalid bucket name: 3-63 characters of lowercase letters, digits, dots or dashes"), 400)
	}
	if err := client.MakeBucket(ctx, name, minio.MakeBucketOptions{Region: cc.region}); err != nil {
		return apierrors.NewError(fmt.Errorf("create bucket failed: %w", err), 502)
	}
	return nil
}

func (c *controller) DeleteBucket(ctx context.Context, datasourceID int64, bucket string) error {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(bucket) == "" {
		return apierrors.NewError(fmt.Errorf("bucket is required"), 400)
	}
	if err := client.RemoveBucket(ctx, bucket); err != nil {
		return apierrors.NewError(fmt.Errorf("delete bucket failed: %w", err), 502)
	}
	return nil
}

// hasErrorCode 判断 S3 错误响应码，用于兼容“配置不存在”类正常场景
func hasErrorCode(err error, codes ...string) bool {
	er := minio.ToErrorResponse(err)
	for _, code := range codes {
		if er.Code == code {
			return true
		}
	}
	return false
}

type policyStatement struct {
	Effect    string      `json:"Effect"`
	Principal interface{} `json:"Principal"`
	Action    interface{} `json:"Action"`
}

type bucketPolicyDoc struct {
	Statement []policyStatement `json:"Statement"`
}

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

// classifyBucketPolicy 将匿名访问策略归类为读写权限级别（对齐云厂商 ACL 展示）
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

func containsAny(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// buildBucketPolicy 生成匿名访问策略，public-read 只读，public-read-write 可读写
func buildBucketPolicy(bucket, acl string) string {
	actions := []string{"s3:GetObject"}
	if acl == "public-read-write" {
		actions = append(actions, "s3:PutObject", "s3:DeleteObject")
	}
	doc := map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Effect":    "Allow",
				"Principal": "*",
				"Action":    actions,
				"Resource":  []string{fmt.Sprintf("arn:aws:s3:::%s/*", bucket)},
			},
		},
	}
	data, _ := json.Marshal(doc)
	return string(data)
}

func (c *controller) GetBucketConfig(ctx context.Context, datasourceID int64, bucket string) (*types.StorageBucketConfig, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(bucket) == "" {
		return nil, apierrors.NewError(fmt.Errorf("bucket is required"), 400)
	}
	cfg := &types.StorageBucketConfig{ACL: "private", Tags: map[string]string{}}

	policyText, err := client.GetBucketPolicy(ctx, bucket)
	if err != nil && !hasErrorCode(err, "NoSuchBucketPolicy") {
		return nil, apierrors.NewError(fmt.Errorf("get bucket policy failed: %w", err), 502)
	}
	cfg.ACL = classifyBucketPolicy(policyText)

	vc, err := client.GetBucketVersioning(ctx, bucket)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("get bucket versioning failed: %w", err), 502)
	}
	cfg.Versioning = vc.Status

	enc, err := client.GetBucketEncryption(ctx, bucket)
	if err == nil {
		cfg.Encryption = enc != nil && len(enc.Rules) > 0
	} else if !hasErrorCode(err, "ServerSideEncryptionConfigurationNotFoundError") {
		return nil, apierrors.NewError(fmt.Errorf("get bucket encryption failed: %w", err), 502)
	}

	bucketTags, err := client.GetBucketTagging(ctx, bucket)
	if err == nil && bucketTags != nil {
		cfg.Tags = bucketTags.ToMap()
	} else if err != nil && !hasErrorCode(err, "NoSuchTagSet") {
		return nil, apierrors.NewError(fmt.Errorf("get bucket tags failed: %w", err), 502)
	}
	return cfg, nil
}

func (c *controller) SetBucketConfig(ctx context.Context, datasourceID int64, bucket string, in *types.StorageBucketConfigUpdate) error {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	bucket = strings.TrimSpace(bucket)
	if bucket == "" || in == nil {
		return apierrors.NewError(fmt.Errorf("bucket and config are required"), 400)
	}
	if in.ACL != nil {
		switch *in.ACL {
		case "private":
			if err := client.SetBucketPolicy(ctx, bucket, ""); err != nil {
				return apierrors.NewError(fmt.Errorf("set bucket policy failed: %w", err), 502)
			}
		case "public-read", "public-read-write":
			if err := client.SetBucketPolicy(ctx, bucket, buildBucketPolicy(bucket, *in.ACL)); err != nil {
				return apierrors.NewError(fmt.Errorf("set bucket policy failed: %w", err), 502)
			}
		default:
			return apierrors.NewError(fmt.Errorf("invalid acl: %s", *in.ACL), 400)
		}
	}
	if in.Versioning != nil {
		status := *in.Versioning
		if status != minio.Enabled && status != minio.Suspended {
			return apierrors.NewError(fmt.Errorf("invalid versioning status: %s", status), 400)
		}
		if err := client.SetBucketVersioning(ctx, bucket, minio.BucketVersioningConfiguration{Status: status}); err != nil {
			return apierrors.NewError(fmt.Errorf("set bucket versioning failed: %w", err), 502)
		}
	}
	if in.Encryption != nil {
		if *in.Encryption {
			if err := client.SetBucketEncryption(ctx, bucket, sse.NewConfigurationSSES3()); err != nil {
				return apierrors.NewError(fmt.Errorf("set bucket encryption failed: %w", err), 502)
			}
		} else if err := client.RemoveBucketEncryption(ctx, bucket); err != nil && !hasErrorCode(err, "ServerSideEncryptionConfigurationNotFoundError") {
			return apierrors.NewError(fmt.Errorf("remove bucket encryption failed: %w", err), 502)
		}
	}
	if in.Tags != nil {
		bucketTags, err := tags.NewTags(in.Tags, false)
		if err != nil {
			return apierrors.NewError(fmt.Errorf("invalid bucket tags: %w", err), 400)
		}
		if err := client.SetBucketTagging(ctx, bucket, bucketTags); err != nil {
			return apierrors.NewError(fmt.Errorf("set bucket tags failed: %w", err), 502)
		}
	}
	return nil
}

// EmptyBucket 递归删除 Bucket 内全部对象（含目录占位对象），用于删除非空 Bucket 前的清空操作
func (c *controller) EmptyBucket(ctx context.Context, datasourceID int64, bucket string) error {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(bucket) == "" {
		return apierrors.NewError(fmt.Errorf("bucket is required"), 400)
	}
	objectsCh := make(chan minio.ObjectInfo, 1000)
	var listErr error
	go func() {
		defer close(objectsCh)
		for item := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true}) {
			if item.Err != nil {
				listErr = item.Err
				return
			}
			objectsCh <- minio.ObjectInfo{Key: item.Key}
		}
	}()
	for removeErr := range client.RemoveObjects(ctx, bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if removeErr.Err != nil {
			return apierrors.NewError(fmt.Errorf("delete object(%s) failed: %w", removeErr.ObjectName, removeErr.Err), 502)
		}
	}
	if listErr != nil {
		return apierrors.NewError(fmt.Errorf("list storage objects failed: %w", listErr), 502)
	}
	return nil
}

func (c *controller) GetLifecycle(ctx context.Context, datasourceID int64, bucket string) ([]types.StorageLifecycleRule, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	cfg, err := client.GetBucketLifecycle(ctx, bucket)
	if err != nil {
		if hasErrorCode(err, "NoSuchLifecycleConfiguration") {
			return []types.StorageLifecycleRule{}, nil
		}
		return nil, apierrors.NewError(fmt.Errorf("get bucket lifecycle failed: %w", err), 502)
	}
	rules := make([]types.StorageLifecycleRule, 0, len(cfg.Rules))
	for _, r := range cfg.Rules {
		rules = append(rules, types.StorageLifecycleRule{
			ID:                 r.ID,
			Status:             r.Status,
			Prefix:             r.RuleFilter.Prefix,
			ExpirationDays:     int(r.Expiration.Days),
			AbortMultipartDays: int(r.AbortIncompleteMultipartUpload.DaysAfterInitiation),
		})
	}
	return rules, nil
}

func (c *controller) SetLifecycle(ctx context.Context, datasourceID int64, bucket string, rules []types.StorageLifecycleRule) error {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(bucket) == "" {
		return apierrors.NewError(fmt.Errorf("bucket is required"), 400)
	}
	cfg := lifecycle.NewConfiguration()
	for i, r := range rules {
		status := r.Status
		if status == "" {
			status = "Enabled"
		}
		if status != "Enabled" && status != "Disabled" {
			return apierrors.NewError(fmt.Errorf("invalid lifecycle rule status: %s", status), 400)
		}
		if r.ExpirationDays <= 0 && r.AbortMultipartDays <= 0 {
			return apierrors.NewError(fmt.Errorf("lifecycle rule requires expiration or abort days"), 400)
		}
		id := r.ID
		if id == "" {
			id = fmt.Sprintf("pixiu-rule-%d", i+1)
		}
		rule := lifecycle.Rule{
			ID:         id,
			Status:     status,
			RuleFilter: lifecycle.Filter{Prefix: r.Prefix},
		}
		if r.ExpirationDays > 0 {
			rule.Expiration = lifecycle.Expiration{Days: lifecycle.ExpirationDays(r.ExpirationDays)}
		}
		if r.AbortMultipartDays > 0 {
			rule.AbortIncompleteMultipartUpload = lifecycle.AbortIncompleteMultipartUpload{DaysAfterInitiation: lifecycle.ExpirationDays(r.AbortMultipartDays)}
		}
		cfg.Rules = append(cfg.Rules, rule)
	}
	// 空规则集合由 minio-go 转换为删除生命周期配置
	if err := client.SetBucketLifecycle(ctx, bucket, cfg); err != nil {
		return apierrors.NewError(fmt.Errorf("set bucket lifecycle failed: %w", err), 502)
	}
	return nil
}

func (c *controller) GetCors(ctx context.Context, datasourceID int64, bucket string) ([]types.StorageCorsRule, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	cfg, err := client.GetBucketCors(ctx, bucket)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("get bucket cors failed: %w", err), 502)
	}
	if cfg == nil {
		return []types.StorageCorsRule{}, nil
	}
	rules := make([]types.StorageCorsRule, 0, len(cfg.CORSRules))
	for _, r := range cfg.CORSRules {
		rules = append(rules, types.StorageCorsRule{
			ID:             r.ID,
			AllowedOrigins: r.AllowedOrigin,
			AllowedMethods: r.AllowedMethod,
			AllowedHeaders: r.AllowedHeader,
			ExposeHeaders:  r.ExposeHeader,
			MaxAgeSeconds:  r.MaxAgeSeconds,
		})
	}
	return rules, nil
}

func (c *controller) SetCors(ctx context.Context, datasourceID int64, bucket string, rules []types.StorageCorsRule) error {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(bucket) == "" {
		return apierrors.NewError(fmt.Errorf("bucket is required"), 400)
	}
	// 空规则集合传 nil，由 minio-go 删除 CORS 配置
	if len(rules) == 0 {
		if err := client.SetBucketCors(ctx, bucket, nil); err != nil && !hasErrorCode(err, "NoSuchCORSConfiguration") {
			return apierrors.NewError(fmt.Errorf("remove bucket cors failed: %w", err), 502)
		}
		return nil
	}
	corsRules := make([]cors.Rule, 0, len(rules))
	for i, r := range rules {
		if len(r.AllowedOrigins) == 0 || len(r.AllowedMethods) == 0 {
			return apierrors.NewError(fmt.Errorf("cors rule requires origins and methods"), 400)
		}
		id := r.ID
		if id == "" {
			id = fmt.Sprintf("pixiu-cors-%d", i+1)
		}
		corsRules = append(corsRules, cors.Rule{
			ID:            id,
			AllowedOrigin: r.AllowedOrigins,
			AllowedMethod: r.AllowedMethods,
			AllowedHeader: r.AllowedHeaders,
			ExposeHeader:  r.ExposeHeaders,
			MaxAgeSeconds: r.MaxAgeSeconds,
		})
	}
	if err := client.SetBucketCors(ctx, bucket, cors.NewConfig(corsRules)); err != nil {
		return apierrors.NewError(fmt.Errorf("set bucket cors failed: %w", err), 502)
	}
	return nil
}

func (c *controller) ListObjectsPage(ctx context.Context, datasourceID int64, bucket, prefix, token string, pageSize int, keyword string, recursive bool) (*types.StorageObjectPage, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, apierrors.NewError(fmt.Errorf("bucket is required"), 400)
	}
	if pageSize <= 0 || pageSize > 500 {
		pageSize = 100
	}
	core := minio.Core{Client: client}

	if recursive {
		// 递归模式：平铺列举全部对象（含目录占位对象），用于导出对象清单等场景
		res, err := core.ListObjectsV2(bucket, prefix, "", token, "", pageSize)
		if err != nil {
			return nil, apierrors.NewError(fmt.Errorf("list storage objects failed: %w", err), 502)
		}
		items := make([]types.StorageObject, 0, len(res.Contents))
		for _, o := range res.Contents {
			items = append(items, types.StorageObject{Key: o.Key, Size: o.Size, LastModified: &o.LastModified, ETag: o.ETag})
		}
		nextToken := ""
		if res.IsTruncated {
			nextToken = res.NextContinuationToken
		}
		return &types.StorageObjectPage{Items: items, NextToken: nextToken, Truncated: res.IsTruncated}, nil
	}

	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		// 搜索模式：递归扫描并按关键字过滤，token 为上一个命中的 key（marker），保证翻页不重不漏
		items := make([]types.StorageObject, 0, pageSize)
		marker := token
		truncated := false
		for {
			res, err := core.ListObjects(bucket, prefix, marker, "", 1000)
			if err != nil {
				return nil, apierrors.NewError(fmt.Errorf("list storage objects failed: %w", err), 502)
			}
			for _, o := range res.Contents {
				if !strings.Contains(strings.ToLower(o.Key), strings.ToLower(keyword)) {
					continue
				}
				items = append(items, types.StorageObject{Key: o.Key, Size: o.Size, LastModified: &o.LastModified, ETag: o.ETag})
				if len(items) >= pageSize {
					break
				}
			}
			if len(items) >= pageSize {
				// 本页已凑满：若本页还有剩余对象或后端未列举完，则存在下一页
				lastKey := items[len(items)-1].Key
				hasMoreInPage := len(res.Contents) > 0 && res.Contents[len(res.Contents)-1].Key > lastKey
				truncated = hasMoreInPage || res.IsTruncated
				marker = lastKey
				break
			}
			if !res.IsTruncated {
				truncated = false
				marker = ""
				break
			}
			marker = res.Contents[len(res.Contents)-1].Key
		}
		if !truncated {
			marker = ""
		}
		return &types.StorageObjectPage{Items: items, NextToken: marker, Truncated: truncated}, nil
	}

	// 目录模式：delimiter 分层列举，CommonPrefixes 作为目录项置顶
	res, err := core.ListObjectsV2(bucket, prefix, "", token, "/", pageSize)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("list storage objects failed: %w", err), 502)
	}
	items := make([]types.StorageObject, 0, len(res.CommonPrefixes)+len(res.Contents))
	for _, cp := range res.CommonPrefixes {
		obj := types.StorageObject{Key: cp.Prefix, IsDir: true}
		// 目录为虚拟前缀，默认无元数据：优先取占位对象修改时间，
		// 隐式目录（无占位对象）则聚合目录下最近一次对象修改时间
		if stat, statErr := client.StatObject(ctx, bucket, cp.Prefix, minio.StatObjectOptions{}); statErr == nil && !stat.LastModified.IsZero() {
			obj.LastModified = &stat.LastModified
		} else if latest, ok := latestObjectTime(core, bucket, cp.Prefix); ok {
			obj.LastModified = &latest
		}
		items = append(items, obj)
	}
	for _, o := range res.Contents {
		if strings.HasSuffix(o.Key, "/") {
			// 忽略模拟目录的占位空对象
			continue
		}
		items = append(items, types.StorageObject{Key: o.Key, Size: o.Size, LastModified: &o.LastModified, ETag: o.ETag})
	}
	nextToken := ""
	if res.IsTruncated {
		nextToken = res.NextContinuationToken
	}
	return &types.StorageObjectPage{Items: items, NextToken: nextToken, Truncated: res.IsTruncated}, nil
}

// latestObjectTime 聚合目录下最近一次对象修改时间；最多扫描 10 页（1 万对象）控制大目录开销
func latestObjectTime(core minio.Core, bucket, prefix string) (time.Time, bool) {
	var latest time.Time
	marker := ""
	for page := 0; page < 10; page++ {
		res, err := core.ListObjects(bucket, prefix, marker, "", 1000)
		if err != nil {
			break
		}
		for _, o := range res.Contents {
			if o.LastModified.After(latest) {
				latest = o.LastModified
			}
		}
		if !res.IsTruncated || len(res.Contents) == 0 {
			break
		}
		marker = res.Contents[len(res.Contents)-1].Key
	}
	return latest, !latest.IsZero()
}

func (c *controller) PutObject(ctx context.Context, datasourceID int64, bucket, key string, reader io.Reader, size int64, contentType string) error {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(bucket) == "" || strings.TrimSpace(key) == "" {
		return apierrors.NewError(fmt.Errorf("bucket and object key are required"), 400)
	}
	if _, err := client.PutObject(ctx, bucket, key, reader, size, minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return apierrors.NewError(fmt.Errorf("upload object failed: %w", err), 502)
	}
	return nil
}

func (c *controller) DeleteObjects(ctx context.Context, datasourceID int64, bucket string, keys []string) error {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(bucket) == "" || len(keys) == 0 {
		return apierrors.NewError(fmt.Errorf("bucket and object keys are required"), 400)
	}
	// 以 / 结尾的 key 视为目录：递归展开为目录下全部对象与占位对象
	expanded := make([]string, 0, len(keys))
	for _, key := range keys {
		expanded = append(expanded, key)
		if !strings.HasSuffix(key, "/") {
			continue
		}
		for item := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: key, Recursive: true}) {
			if item.Err != nil {
				return apierrors.NewError(fmt.Errorf("list storage objects failed: %w", item.Err), 502)
			}
			expanded = append(expanded, item.Key)
		}
	}
	objectsCh := make(chan minio.ObjectInfo, len(expanded))
	for _, key := range expanded {
		objectsCh <- minio.ObjectInfo{Key: key}
	}
	close(objectsCh)
	for removeErr := range client.RemoveObjects(ctx, bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if removeErr.Err != nil {
			return apierrors.NewError(fmt.Errorf("delete object(%s) failed: %w", removeErr.ObjectName, removeErr.Err), 502)
		}
	}
	return nil
}

func (c *controller) RenameObject(ctx context.Context, datasourceID int64, bucket, srcKey, dstKey string) error {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(bucket) == "" || strings.TrimSpace(srcKey) == "" || strings.TrimSpace(dstKey) == "" {
		return apierrors.NewError(fmt.Errorf("bucket and object keys are required"), 400)
	}
	if srcKey == dstKey {
		return apierrors.NewError(fmt.Errorf("source and destination key are identical"), 400)
	}
	if _, err := client.CopyObject(ctx, minio.CopyDestOptions{Bucket: bucket, Object: dstKey}, minio.CopySrcOptions{Bucket: bucket, Object: srcKey}); err != nil {
		return apierrors.NewError(fmt.Errorf("copy object failed: %w", err), 502)
	}
	if err := client.RemoveObject(ctx, bucket, srcKey, minio.RemoveObjectOptions{}); err != nil {
		return apierrors.NewError(fmt.Errorf("remove source object failed: %w", err), 502)
	}
	return nil
}

// adminClientFor builds a MinIO admin client for user and policy management.
// The admin API only exists on MinIO servers, so non-MinIO providers are rejected early.
func (c *controller) adminClientFor(ctx context.Context, datasourceID int64) (*madmin.AdminClient, error) {
	_, cc, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	switch cc.provider {
	case "", "s3", "minio":
	default:
		return nil, apierrors.NewError(fmt.Errorf("user management is only supported for MinIO compatible datasources"), 400)
	}
	u, err := url.Parse(cc.endpoint)
	if err != nil || u.Host == "" {
		return nil, apierrors.NewError(fmt.Errorf("invalid storage endpoint"), 400)
	}
	adminClient, err := madmin.New(u.Host, cc.access, cc.secret, cc.secure)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("create storage admin client: %w", err), 400)
	}
	return adminClient, nil
}

// formatPolicyNames renders the raw policy field of a MinIO user (a JSON string or array) as plain names.
func formatPolicyNames(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "[") {
		var names []string
		if err := json.Unmarshal([]byte(trimmed), &names); err == nil {
			return strings.Join(names, ", ")
		}
		return trimmed
	}
	var name string
	if err := json.Unmarshal([]byte(trimmed), &name); err == nil {
		return name
	}
	return trimmed
}

func (c *controller) ListUsers(ctx context.Context, datasourceID int64) ([]types.StorageUser, error) {
	adminClient, err := c.adminClientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	users, err := adminClient.ListUsers(ctx)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("list storage users failed: %w", err), 502)
	}
	result := make([]types.StorageUser, 0, len(users))
	for accessKey, detail := range users {
		result = append(result, types.StorageUser{
			AccessKey: accessKey,
			Status:    string(detail.Status),
			Policy:    formatPolicyNames(detail.PolicyName),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AccessKey < result[j].AccessKey })
	return result, nil
}

func (c *controller) CreateUser(ctx context.Context, datasourceID int64, in *types.StorageUserCreate) error {
	adminClient, err := c.adminClientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	accessKey := strings.TrimSpace(in.AccessKey)
	if len(accessKey) < 3 {
		return apierrors.NewError(fmt.Errorf("access key must be at least 3 characters"), 400)
	}
	if len(in.SecretKey) < 8 {
		return apierrors.NewError(fmt.Errorf("secret key must be at least 8 characters"), 400)
	}
	if err := adminClient.AddUser(ctx, accessKey, in.SecretKey); err != nil {
		return apierrors.NewError(fmt.Errorf("create storage user failed: %w", err), 502)
	}
	if policy := strings.TrimSpace(in.Policy); policy != "" {
		if err := adminClient.SetPolicy(ctx, policy, accessKey, false); err != nil {
			return apierrors.NewError(fmt.Errorf("user created but attach policy failed: %w", err), 502)
		}
	}
	return nil
}

func (c *controller) DeleteUser(ctx context.Context, datasourceID int64, accessKey string) error {
	adminClient, err := c.adminClientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(accessKey) == "" {
		return apierrors.NewError(fmt.Errorf("access key is required"), 400)
	}
	if err := adminClient.RemoveUser(ctx, accessKey); err != nil {
		return apierrors.NewError(fmt.Errorf("delete storage user failed: %w", err), 502)
	}
	return nil
}

func (c *controller) ListPolicies(ctx context.Context, datasourceID int64) ([]string, error) {
	adminClient, err := c.adminClientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	policies, err := adminClient.ListCannedPolicies(ctx)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("list storage policies failed: %w", err), 502)
	}
	result := make([]string, 0, len(policies))
	for name := range policies {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

// GetOverview aggregates instance level monitoring data. MinIO admin APIs provide
// server info and data usage; other providers degrade to basic bucket statistics.
func (c *controller) GetOverview(ctx context.Context, datasourceID int64) (*types.StorageOverview, error) {
	client, cc, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	overview := &types.StorageOverview{Provider: cc.provider}
	bucketList, err := client.ListBuckets(ctx)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("list buckets failed: %w", err), 502)
	}
	overview.Buckets = len(bucketList)

	adminClient, err := c.adminClientFor(ctx, datasourceID)
	if err != nil {
		return overview, nil
	}
	overview.Minio = true
	if info, infoErr := adminClient.ServerInfo(ctx, madmin.WithDriveMetrics(true)); infoErr == nil {
		overview.Mode = info.Mode
		overview.Region = info.Region
		overview.DeploymentID = info.DeploymentID
		overview.Domains = info.Domain
		overview.VersionsCount = info.Versions.Count
		overview.DeleteMarkersCnt = info.DeleteMarkers.Count
		overview.OnlineDisks = info.Backend.OnlineDisks
		overview.OfflineDisks = info.Backend.OfflineDisks
		overview.Backend = toStorageBackendInfo(info.Backend)
		overview.Encryption = resolveEncryption(info.Services)
		overview.ErasureSets = collectErasureSets(info.Pools)
		for _, server := range info.Servers {
			if overview.Version == "" && server.Version != "" {
				overview.Version = server.Version
				overview.Uptime = server.Uptime
			}
			if overview.Edition == "" {
				overview.Edition = server.Edition
			}
			disks := make([]types.StorageDiskInfo, 0, len(server.Disks))
			for _, disk := range server.Disks {
				disks = append(disks, toStorageDiskInfo(disk))
			}
			overview.Servers = append(overview.Servers, types.StorageServerInfo{
				Endpoint:       server.Endpoint,
				State:          server.State,
				Version:        server.Version,
				Uptime:         server.Uptime,
				MemAlloc:       server.MemStats.Alloc,
				HeapAlloc:      server.MemStats.HeapAlloc,
				NumCPU:         server.NumCPU,
				RuntimeVersion: server.RuntimeVersion,
				PoolNumbers:    server.PoolNumbers,
				Disks:          disks,
			})
		}
	}
	if usage, usageErr := adminClient.DataUsageInfo(ctx); usageErr == nil {
		overview.Objects = usage.ObjectsTotalCount
		overview.UsedBytes = usage.TotalUsedCapacity
		overview.TotalBytes = usage.TotalCapacity
		overview.FreeBytes = usage.TotalFreeCapacity
		if !usage.LastUpdate.IsZero() {
			overview.UpdatedAt = usage.LastUpdate.Format("2006-01-02 15:04:05")
		}
		items := make([]types.StorageBucketUsage, 0, len(usage.BucketsUsage))
		histogram := map[string]uint64{}
		for name, bu := range usage.BucketsUsage {
			items = append(items, types.StorageBucketUsage{
				Name:             name,
				Objects:          bu.ObjectsCount,
				Bytes:            bu.Size,
				VersionsCount:    bu.VersionsCount,
				DeleteMarkersCnt: bu.DeleteMarkersCount,
			})
			for bin, count := range bu.ObjectSizesHistogram {
				histogram[bin] += count
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Bytes > items[j].Bytes })
		overview.BucketUsage = items
		overview.ObjectHistogram = aggregateObjectHistogram(histogram)
		overview.Replication = toStorageReplicationInfo(usage)
		overview.Tiers = collectTierStats(usage.TierStats)
	}
	// Single-drive deployments report no cluster capacity; fall back to drive sums.
	if overview.TotalBytes == 0 {
		if si, siErr := adminClient.StorageInfo(ctx); siErr == nil {
			var total, used uint64
			for _, disk := range si.Disks {
				total += disk.TotalSpace
				used += disk.UsedSpace
			}
			overview.TotalBytes = total
			overview.UsedBytes = used
			if total > used {
				overview.FreeBytes = total - used
			}
			// 单盘部署 ServerInfo 可能不带磁盘明细，这里回填到唯一节点供前端展示
			if len(overview.Servers) == 0 {
				overview.Servers = []types.StorageServerInfo{{State: "ok"}}
			}
			if len(overview.Servers) == 1 && len(overview.Servers[0].Disks) == 0 {
				disks := make([]types.StorageDiskInfo, 0, len(si.Disks))
				for _, disk := range si.Disks {
					disks = append(disks, toStorageDiskInfo(disk))
				}
				overview.Servers[0].Disks = disks
			}
		}
	}
	return overview, nil
}

func toStorageDiskInfo(disk madmin.Disk) types.StorageDiskInfo {
	return types.StorageDiskInfo{
		Path:            disk.DrivePath,
		State:           disk.State,
		Model:           disk.Model,
		TotalSpace:      disk.TotalSpace,
		UsedSpace:       disk.UsedSpace,
		AvailableSpace:  disk.AvailableSpace,
		ReadThroughput:  disk.ReadThroughput,
		WriteThroughput: disk.WriteThroughPut,
		ReadLatency:     disk.ReadLatency,
		WriteLatency:    disk.WriteLatency,
		Utilization:     disk.Utilization,
		UsedInodes:      disk.UsedInodes,
		FreeInodes:      disk.FreeInodes,
		Healing:         disk.Healing,
		Scanning:        disk.Scanning,
		RootDisk:        disk.RootDisk,
	}
}

// resolveEncryption reports whether server-side encryption via KMS is usable.
func resolveEncryption(services madmin.Services) string {
	if len(services.KMSStatus) == 0 {
		return "disabled"
	}
	for _, kms := range services.KMSStatus {
		if strings.EqualFold(kms.Status, "ok") && strings.EqualFold(kms.Encrypt, "ok") {
			return "enabled"
		}
	}
	return "error"
}

// collectErasureSets flattens InfoMessage.Pools (pool -> set -> info) into an
// ordered slice for the overview API.
func collectErasureSets(pools map[int]map[int]madmin.ErasureSetInfo) []types.StorageErasureSetInfo {
	if len(pools) == 0 {
		return nil
	}
	poolIDs := make([]int, 0, len(pools))
	for poolID := range pools {
		poolIDs = append(poolIDs, poolID)
	}
	sort.Ints(poolIDs)
	sets := make([]types.StorageErasureSetInfo, 0)
	for _, poolID := range poolIDs {
		setIDs := make([]int, 0, len(pools[poolID]))
		for setID := range pools[poolID] {
			setIDs = append(setIDs, setID)
		}
		sort.Ints(setIDs)
		for _, setID := range setIDs {
			set := pools[poolID][setID]
			sets = append(sets, types.StorageErasureSetInfo{
				Pool:        poolID,
				SetID:       set.ID,
				Usage:       set.Usage,
				RawCapacity: set.RawCapacity,
				Objects:     set.ObjectsCount,
				HealDisks:   set.HealDisks,
			})
		}
	}
	return sets
}

// toStorageReplicationInfo returns nil when the deployment has no replication
// traffic at all, so the UI can hide the panel.
func toStorageReplicationInfo(usage madmin.DataUsageInfo) *types.StorageReplicationInfo {
	info := &types.StorageReplicationInfo{
		PendingCount:   usage.ReplicationPendingCount,
		FailedCount:    usage.ReplicationFailedCount,
		PendingSize:    usage.ReplicationPendingSize,
		FailedSize:     usage.ReplicationFailedSize,
		ReplicatedSize: usage.ReplicatedSize,
		ReplicaSize:    usage.ReplicaSize,
	}
	if info.PendingCount == 0 && info.FailedCount == 0 && info.ReplicatedSize == 0 && info.ReplicaSize == 0 {
		return nil
	}
	return info
}

func collectTierStats(stats map[string]madmin.TierStats) []types.StorageTierStats {
	if len(stats) == 0 {
		return nil
	}
	names := make([]string, 0, len(stats))
	for name := range stats {
		names = append(names, name)
	}
	sort.Strings(names)
	tiers := make([]types.StorageTierStats, 0, len(names))
	for _, name := range names {
		tier := stats[name]
		if tier.TotalSize == 0 && tier.NumObjects == 0 {
			continue
		}
		tiers = append(tiers, types.StorageTierStats{
			Name:        name,
			TotalSize:   tier.TotalSize,
			NumObjects:  tier.NumObjects,
			NumVersions: tier.NumVersions,
		})
	}
	return tiers
}

// toStorageBackendInfo converts the madmin erasure/FS backend info into the
// API shape. Data shards are derived from drives per set minus parity disks.
func toStorageBackendInfo(backend madmin.ErasureBackend) *types.StorageBackendInfo {
	info := &types.StorageBackendInfo{
		StdParity: backend.StandardSCParity,
	}
	switch backend.Type {
	case madmin.FsType:
		info.Type = "fs"
	case madmin.ErasureType:
		info.Type = "erasure"
	default:
		return nil
	}
	if len(backend.TotalSets) > 0 {
		info.Sets = backend.TotalSets[0]
	}
	if len(backend.DrivesPerSet) > 0 {
		info.DrivesPerSet = backend.DrivesPerSet[0]
	}
	if info.DrivesPerSet > 0 && info.StdParity > 0 && info.DrivesPerSet > info.StdParity {
		info.StdData = info.DrivesPerSet - info.StdParity
	}
	return info
}

// aggregateObjectHistogram merges per-bucket object-size histograms into an
// ordered distribution. Bin keys follow the MinIO server convention
// ("10", "100", "1_KiB", ..., "1_TiB", "big"); unknown keys keep their raw label.
var objectHistogramOrder = map[string]int{
	"10": 0, "100": 1, "1_KiB": 2, "10_KiB": 3, "100_KiB": 4,
	"1_MiB": 5, "10_MiB": 6, "100_MiB": 7, "1_GiB": 8, "10_GiB": 9,
	"1_TiB": 10, "big": 11,
}

func aggregateObjectHistogram(histogram map[string]uint64) []types.StorageHistogramBin {
	keys := make([]string, 0, len(histogram))
	for bin := range histogram {
		if histogram[bin] == 0 {
			continue
		}
		keys = append(keys, bin)
	}
	sort.Slice(keys, func(i, j int) bool {
		oi, oj := objectHistogramOrder[keys[i]], objectHistogramOrder[keys[j]]
		if oi == oj {
			return keys[i] < keys[j]
		}
		return oi < oj
	})
	bins := make([]types.StorageHistogramBin, 0, len(keys))
	for _, bin := range keys {
		bins = append(bins, types.StorageHistogramBin{
			Label: objectHistogramLabel(bin),
			Count: histogram[bin],
		})
	}
	return bins
}

func objectHistogramLabel(bin string) string {
	if bin == "big" {
		return ">1 TiB"
	}
	if strings.Contains(bin, "_") {
		return "≤" + strings.ReplaceAll(bin, "_", " ")
	}
	return "≤" + bin + " B"
}
