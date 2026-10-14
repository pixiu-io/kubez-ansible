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
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
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
	PingAdhocWithDatasource(context.Context, int64, *types.StorageSourceConfig) (*types.StoragePing, error)
	Ping(context.Context, int64) (*types.StoragePing, error)
	ListBuckets(context.Context, int64) ([]types.StorageBucket, error)
	CreateBucket(context.Context, int64, string) error
	DeleteBucket(context.Context, int64, string) error
	GetBucketStats(context.Context, int64, string) (*types.StorageBucketStats, error)
	GetBucketConfig(context.Context, int64, string) (*types.StorageBucketConfig, error)
	SetBucketConfig(context.Context, int64, string, *types.StorageBucketConfigUpdate) error
	EmptyBucket(context.Context, int64, string) error
	GetLifecycle(context.Context, int64, string) ([]types.StorageLifecycleRule, error)
	SetLifecycle(context.Context, int64, string, []types.StorageLifecycleRule) error
	GetCors(context.Context, int64, string) ([]types.StorageCorsRule, error)
	SetCors(context.Context, int64, string, []types.StorageCorsRule) error
	ListObjectsPage(context.Context, int64, string, string, string, int, string, bool) (*types.StorageObjectPage, error)
	PutObject(context.Context, int64, string, string, io.Reader, int64, string) error
	DeleteObjects(context.Context, int64, string, []string) error
	RenameObject(context.Context, int64, string, string, string) error
	ListObjectVersions(context.Context, int64, string, string, bool, int) (*types.StorageObjectVersionList, error)
	GetBucketVersioningState(context.Context, int64, string) (string, error)
	RestoreObjectVersion(context.Context, int64, string, string, string) error
	DeleteObjectVersion(context.Context, int64, string, string, string) error
	ListIncompleteUploads(context.Context, int64, string, string) (*types.StorageMultipartUploadList, error)
	AbortIncompleteUploads(context.Context, int64, string, []string) (*types.StorageAbortUploadResult, error)
	// 大文件分片上传（S3 Multipart Upload）
	InitMultipartUpload(context.Context, int64, *types.StorageMultipartInit) (*types.StorageMultipartSession, error)
	UploadMultipartPart(context.Context, int64, string, string, string, int, io.Reader, int64) (*types.StorageMultipartPart, error)
	ListMultipartParts(context.Context, int64, string, string, string) ([]types.StorageMultipartPart, error)
	CompleteMultipartUpload(context.Context, int64, *types.StorageMultipartComplete) (*types.StorageMultipartResult, error)
	AbortMultipartUpload(context.Context, int64, *types.StorageMultipartTarget) error
	GetObjectTags(context.Context, int64, string, string, string) ([]types.StorageObjectTag, error)
	SetObjectTags(context.Context, int64, string, string, string, []types.StorageObjectTag) error
	GetObjectDetail(context.Context, int64, string, string, string) (*types.StorageObjectDetail, error)
	UpdateObjectMeta(context.Context, int64, string, string, *types.StorageObjectMetaUpdate) error
	OpenObject(context.Context, int64, string, string) (io.ReadCloser, int64, string, error)
	PresignedGetObject(context.Context, int64, string, string, int) (*types.StoragePresignedObject, error)
	// Bucket 授权策略（完整 JSON）与对象跨桶复制/移动、批量打包下载
	GetBucketPolicy(context.Context, int64, string) (*types.StorageBucketPolicy, error)
	SetBucketPolicy(context.Context, int64, string, string) error
	TransferObjects(context.Context, int64, *types.StorageTransferRequest) (*types.StorageTransferResult, error)
	DownloadObjectsZip(context.Context, int64, string, []string, io.Writer) (bool, error)
	ListUsers(context.Context, int64) ([]types.StorageUser, error)
	CreateUser(context.Context, int64, *types.StorageUserCreate) error
	DeleteUser(context.Context, int64, string) error
	ListPolicies(context.Context, int64) ([]string, error)
	// 厂商控制面能力：供前端展示，不建立任何连接
	ListProviderCapabilities(context.Context) []types.StorageProviderInfo
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

type clientConfig struct {
	provider Provider
	endpoint string
	region   string
	appID    string
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
	provider := providerFor(sc.Provider)
	endpoint := strings.TrimSpace(sc.Endpoint)
	if endpoint == "" && cfg.Log != nil {
		endpoint = strings.TrimSpace(cfg.Log.URL)
	}
	appID := provider.AppID(endpoint)
	if endpoint == "" {
		return nil, clientConfig{}, apierrors.NewError(fmt.Errorf("storage endpoint is required"), 400)
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, clientConfig{}, apierrors.NewError(fmt.Errorf("invalid storage endpoint"), 400)
	}
	access, secret, token := sc.AccessKeyID, sc.SecretAccessKey, sc.SessionToken
	// 兼容存量数据：地址与凭据曾统一落在通用 log 配置里
	if access == "" && cfg.Log != nil {
		access, secret = cfg.Log.UserName, cfg.Log.Password
	}
	if access == "" || secret == "" {
		return nil, clientConfig{}, apierrors.NewError(fmt.Errorf("storage access key and secret key are required"), 400)
	}
	region := strings.TrimSpace(sc.Region)
	if region == "" {
		if provider.RequiresExplicitRegion() {
			return nil, clientConfig{}, apierrors.NewError(fmt.Errorf("storage region is required for provider %s", provider.Key()), 400)
		}
		region = storageDefaultRegion
	}
	mc, err := minio.New(u.Host, &minio.Options{
		Creds:        newStorageCredentials(sc.SignatureVersion, access, secret, token),
		Secure:       u.Scheme == "https",
		Region:       region,
		BucketLookup: provider.BucketLookup(sc.AddressingStyle, u.Hostname()),
	})
	if err != nil {
		return nil, clientConfig{}, apierrors.NewError(fmt.Errorf("create storage client: %w", err), 400)
	}
	return mc, clientConfig{provider: provider, endpoint: endpoint, region: region, appID: appID, access: access, secret: secret, token: token, secure: u.Scheme == "https"}, nil
}

// newStorageCredentials 按签名版本构造凭据。S3 V2 不支持临时令牌，传 token 会被忽略。
func newStorageCredentials(version, access, secret, token string) *credentials.Credentials {
	switch strings.ToLower(strings.TrimSpace(version)) {
	case "v2", "s3v2":
		return credentials.NewStaticV2(access, secret, "")
	default:
		return credentials.NewStaticV4(access, secret, token)
	}
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
	return &types.StoragePing{Connected: true, Provider: cc.provider.Key()}, nil
}

// ListProviderCapabilities 返回各厂商的控制面能力，供前端按实例厂商展示功能入口。
func (c *controller) ListProviderCapabilities(context.Context) []types.StorageProviderInfo {
	return ProviderCapabilities()
}

func (c *controller) PingAdhoc(ctx context.Context, sc *types.StorageSourceConfig) (*types.StoragePing, error) {
	// 临时连接测试会由服务端主动发起连接，是最典型的 SSRF 面：
	// 任意登录用户都能借它探测内网服务的连通性差异。这里叠加按用户的频次限制。
	userName, err := controllerutil.UserNameFromContext(ctx)
	if err != nil {
		userName = "anonymous"
	}
	if !allowStorageProbe(userName) {
		return nil, apierrors.NewError(fmt.Errorf("too many connection probes, please retry later"), 429)
	}
	client, provider, err := clientFromConfig(sc)
	if err != nil {
		return nil, err
	}
	if _, err = client.ListBuckets(ctx); err != nil {
		return nil, apierrors.NewError(fmt.Errorf("storage ping failed: %w", err), 502)
	}
	return &types.StoragePing{Connected: true, Provider: provider.Key()}, nil
}

// PingAdhocWithDatasource uses the current form values and restores the saved secret when it is omitted.
// The API never returns secret keys to the browser, so edit-form tests need this server-side fallback.
func (c *controller) PingAdhocWithDatasource(ctx context.Context, datasourceID int64, sc *types.StorageSourceConfig) (*types.StoragePing, error) {
	if datasourceID <= 0 {
		return c.PingAdhoc(ctx, sc)
	}

	object, err := c.factory.Datasource().Get(ctx, datasourceID)
	if err != nil {
		klog.Errorf("failed to get datasource(%d): %v", datasourceID, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("datasource not found"), 404)
	}
	if object.Type != model.DatasourceTypeMiddleware || object.SubType != model.DatasourceSubTypeStorage {
		return nil, apierrors.NewError(fmt.Errorf("datasource(%d) is not a storage datasource", datasourceID), 400)
	}
	if err = controllerutil.CheckResourceAccess(ctx, c.factory, object.UserId, types.ResourceTypeDatasource, datasourceID); err != nil {
		return nil, err
	}

	var saved types.DatasourceConfig
	if err = saved.Unmarshal(object.Config); err != nil || saved.Storage == nil {
		return nil, apierrors.NewError(fmt.Errorf("invalid storage datasource config"), 400)
	}
	if sc == nil {
		sc = &types.StorageSourceConfig{}
	}
	if sc.SecretAccessKey == "" {
		sc.SecretAccessKey = saved.Storage.SecretAccessKey
		if sc.SecretAccessKey == "" && saved.Log != nil {
			sc.SecretAccessKey = saved.Log.Password
		}
	}
	if sc.SessionToken == "" {
		sc.SessionToken = saved.Storage.SessionToken
	}
	if sc.SignatureVersion == "" {
		sc.SignatureVersion = saved.Storage.SignatureVersion
	}
	if sc.AddressingStyle == "" {
		sc.AddressingStyle = saved.Storage.AddressingStyle
	}
	return c.PingAdhoc(ctx, sc)
}

func clientFromConfig(sc *types.StorageSourceConfig) (*minio.Client, Provider, error) {
	if sc == nil {
		return nil, nil, apierrors.NewError(fmt.Errorf("storage config is required"), 400)
	}
	provider := providerFor(sc.Provider)
	endpoint := strings.TrimSpace(sc.Endpoint)
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, nil, apierrors.NewError(fmt.Errorf("invalid storage endpoint"), 400)
	}
	if err := validateProbeEndpoint(u); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(sc.AccessKeyID) == "" || sc.SecretAccessKey == "" {
		return nil, nil, apierrors.NewError(fmt.Errorf("storage access key and secret key are required"), 400)
	}
	region := strings.TrimSpace(sc.Region)
	if region == "" {
		if provider.RequiresExplicitRegion() {
			return nil, nil, apierrors.NewError(fmt.Errorf("storage region is required for provider %s", provider.Key()), 400)
		}
		region = storageDefaultRegion
	}
	client, err := minio.New(u.Host, &minio.Options{
		Creds:        newStorageCredentials(sc.SignatureVersion, sc.AccessKeyID, sc.SecretAccessKey, sc.SessionToken),
		Secure:       u.Scheme == "https",
		Region:       region,
		BucketLookup: provider.BucketLookup(sc.AddressingStyle, u.Hostname()),
		Transport:    probeHTTPTransport(),
	})
	if err != nil {
		return nil, nil, apierrors.NewError(fmt.Errorf("create storage client: %w", err), 400)
	}
	return client, provider, nil
}

// validateProbeEndpoint 拦截临时连接测试中的 SSRF 高价值目标。
// 只拒绝回环、链路本地（含 169.254.169.254）、组播与未指定地址；不拦私有网段——自建 MinIO 就在内网。
func validateProbeEndpoint(u *url.URL) error {
	host := strings.TrimSpace(u.Hostname())
	if err := validateProbeHost(host); err != nil {
		return err
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return apierrors.NewError(fmt.Errorf("cannot resolve storage endpoint host: %s", host), 400)
	}
	for _, ip := range ips {
		if disallowedProbeIP(ip) {
			return apierrors.NewError(
				fmt.Errorf("storage endpoint %s resolves to a disallowed address (%s)", host, ip.String()), 400)
		}
	}
	return nil
}

func validateProbeHost(host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return apierrors.NewError(fmt.Errorf("invalid storage endpoint"), 400)
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return apierrors.NewError(fmt.Errorf("storage endpoint host %s is not allowed", host), 400)
	}
	return nil
}

func disallowedProbeIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}

// probeHTTPTransport 在每次建连时重新解析并校验目标地址，堵住「校验时解析到公网、拨号时被重绑定到回环」的窗口。
func probeHTTPTransport() http.RoundTripper {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if err := validateProbeHost(host); err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil || len(ips) == 0 {
				return nil, fmt.Errorf("cannot resolve storage endpoint host: %s", host)
			}
			var lastErr error
			tried := false
			for _, ip := range ips {
				if disallowedProbeIP(ip) {
					lastErr = fmt.Errorf("storage endpoint %s resolves to a disallowed address (%s)", host, ip)
					continue
				}
				tried = true
				conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if dialErr == nil {
					return conn, nil
				}
				lastErr = dialErr
			}
			if !tried && lastErr == nil {
				lastErr = fmt.Errorf("storage endpoint host %s is not allowed", host)
			}
			return nil, lastErr
		},
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
}

const (
	// storageProbeWindow 临时连接测试的限流窗口
	storageProbeWindow = time.Minute
	// storageProbeMaxPerWindow 单个用户在窗口内允许的探测次数
	storageProbeMaxPerWindow = 30
)

var storageProbeRecords = struct {
	sync.Mutex
	seen map[string][]time.Time
}{seen: make(map[string][]time.Time)}

// allowStorageProbe 按用户做滑动窗口限流，避免临时连接测试被当作内网端口扫描器使用。
func allowStorageProbe(userName string) bool {
	now := time.Now()
	storageProbeRecords.Lock()
	defer storageProbeRecords.Unlock()
	kept := make([]time.Time, 0, len(storageProbeRecords.seen[userName]))
	for _, at := range storageProbeRecords.seen[userName] {
		if now.Sub(at) < storageProbeWindow {
			kept = append(kept, at)
		}
	}
	if len(kept) >= storageProbeMaxPerWindow {
		storageProbeRecords.seen[userName] = kept
		return false
	}
	storageProbeRecords.seen[userName] = append(kept, now)
	return true
}

func (c *controller) ListBuckets(ctx context.Context, datasourceID int64) ([]types.StorageBucket, error) {
	client, cc, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	buckets, err := client.ListBuckets(ctx)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("list storage buckets failed: %w", err), 502)
	}
	result := make([]types.StorageBucket, 0, len(buckets))
	for _, item := range buckets {
		result = append(result, types.StorageBucket{Name: item.Name, CreationDate: item.CreationDate, Region: cc.region})
	}
	return result, nil
}

// GetBucketStats 递归列举当前 Bucket 的对象统计用量，按需调用，避免列表刷新时全量扫描。
func (c *controller) GetBucketStats(ctx context.Context, datasourceID int64, bucket string) (*types.StorageBucketStats, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, apierrors.NewError(fmt.Errorf("bucket is required"), 400)
	}
	stats := &types.StorageBucketStats{}
	for object := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true}) {
		if object.Err != nil {
			return nil, apierrors.NewError(fmt.Errorf("list bucket objects for stats failed: %w", object.Err), 502)
		}
		stats.Objects++
		if object.Size > 0 {
			stats.Bytes += uint64(object.Size)
		}
	}
	return stats, nil
}

// 临时下载地址的有效期：请求值夹取到 (0, maxPresignExpirySeconds]，缺省 15 分钟；返回的是生效值。
const (
	defaultPresignExpirySeconds = 900
	maxPresignExpirySeconds     = 3600
)

// OpenObject opens an object stream for same-origin consumers such as the preview dialog, so browser
// fetches of public-cloud presigned URLs do not require the Bucket CORS configuration.
func (c *controller) OpenObject(ctx context.Context, datasourceID int64, bucket, objectKey string) (io.ReadCloser, int64, string, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, 0, "", err
	}
	if strings.TrimSpace(bucket) == "" || strings.TrimSpace(objectKey) == "" {
		return nil, 0, "", apierrors.NewError(fmt.Errorf("bucket and object key are required"), 400)
	}
	object, err := client.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, "", apierrors.NewError(fmt.Errorf("open object failed: %w", err), 502)
	}
	info, err := object.Stat()
	if err != nil {
		_ = object.Close()
		return nil, 0, "", apierrors.NewError(fmt.Errorf("stat object failed: %w", err), 502)
	}
	contentType := strings.TrimSpace(info.ContentType)
	if contentType == "" {
		contentType = strings.TrimSpace(info.Metadata.Get("Content-Type"))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return object, info.Size, contentType, nil
}

func (c *controller) PresignedGetObject(ctx context.Context, datasourceID int64, bucket, objectKey string, expirySeconds int) (*types.StoragePresignedObject, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(bucket) == "" || strings.TrimSpace(objectKey) == "" {
		return nil, apierrors.NewError(fmt.Errorf("bucket and object key are required"), 400)
	}
	if expirySeconds <= 0 || expirySeconds > maxPresignExpirySeconds {
		expirySeconds = defaultPresignExpirySeconds
	}
	presigned, err := client.PresignedGetObject(ctx, bucket, objectKey, time.Duration(expirySeconds)*time.Second, nil)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("create presigned url failed: %w", err), 502)
	}
	return &types.StoragePresignedObject{URL: presigned.String(), Expires: expirySeconds}, nil
}

// bucketNameRegex S3 Bucket 命名规范：3-63 位小写字母、数字、点、连字符，首尾为字母或数字
var bucketNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

// S3 明确禁止、但上面字符集放行的两类名称，需单独排除（前端有同款校验，见 browse/index.vue）
var (
	bucketNameDots = regexp.MustCompile(`\.\.`)
	bucketNameIP   = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
)

// validBucketName 校验 Bucket 名称：字符集 + 长度 + 禁止连续点 + 禁止 IP 形式
func validBucketName(name string) bool {
	if !bucketNameRegex.MatchString(name) {
		return false
	}
	return !bucketNameDots.MatchString(name) && !bucketNameIP.MatchString(name)
}

func (c *controller) CreateBucket(ctx context.Context, datasourceID int64, name string) error {
	client, cc, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	name, err = cc.provider.NormalizeBucketName(cc.appID, name)
	if err != nil {
		return apierrors.NewError(err, 400)
	}
	if !validBucketName(name) {
		return apierrors.NewError(
			fmt.Errorf("invalid bucket name: 3-63 characters of lowercase letters, digits, dots or dashes, without consecutive dots or IP-like names"),
			400)
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

func (c *controller) GetBucketConfig(ctx context.Context, datasourceID int64, bucket string) (*types.StorageBucketConfig, error) {
	client, cc, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(bucket) == "" {
		return nil, apierrors.NewError(fmt.Errorf("bucket is required"), 400)
	}
	caps := cc.provider.Capabilities()
	cfg := &types.StorageBucketConfig{Tags: map[string]string{}}

	if caps.BucketPolicy {
		policyText, err := client.GetBucketPolicy(ctx, bucket)
		if err != nil && !hasErrorCode(err, "NoSuchBucketPolicy") {
			return nil, apierrors.NewError(fmt.Errorf("get bucket policy failed: %w", err), 502)
		}
		cfg.ACL = classifyBucketPolicy(policyText)
	}

	vc, err := client.GetBucketVersioning(ctx, bucket)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("get bucket versioning failed: %w", err), 502)
	}
	cfg.Versioning = vc.Status

	if caps.SSEEncryption {
		enc, err := client.GetBucketEncryption(ctx, bucket)
		if err == nil {
			cfg.Encryption = enc != nil && len(enc.Rules) > 0
		} else if !hasErrorCode(err, "ServerSideEncryptionConfigurationNotFoundError") {
			return nil, apierrors.NewError(fmt.Errorf("get bucket encryption failed: %w", err), 502)
		}
	}

	if caps.BucketTagging {
		bucketTags, err := client.GetBucketTagging(ctx, bucket)
		if err == nil && bucketTags != nil {
			cfg.Tags = bucketTags.ToMap()
		} else if err != nil && !hasErrorCode(err, "NoSuchTagSet") {
			return nil, apierrors.NewError(fmt.Errorf("get bucket tags failed: %w", err), 502)
		}
	}
	return cfg, nil
}

func (c *controller) SetBucketConfig(ctx context.Context, datasourceID int64, bucket string, in *types.StorageBucketConfigUpdate) error {
	client, cc, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	bucket = strings.TrimSpace(bucket)
	if bucket == "" || in == nil {
		return apierrors.NewError(fmt.Errorf("bucket and config are required"), 400)
	}
	caps := cc.provider.Capabilities()
	if in.ACL != nil {
		if !caps.BucketPolicy {
			return apierrors.NewError(fmt.Errorf("bucket policy management is not supported for provider %s; use the cloud provider console", cc.provider.Key()), 400)
		}
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
		if !caps.SSEEncryption {
			return apierrors.NewError(fmt.Errorf("SSE-S3 encryption management is not supported for provider %s; use the cloud provider console", cc.provider.Key()), 400)
		}
		if *in.Encryption {
			if err := client.SetBucketEncryption(ctx, bucket, sse.NewConfigurationSSES3()); err != nil {
				return apierrors.NewError(fmt.Errorf("set bucket encryption failed: %w", err), 502)
			}
		} else if err := client.RemoveBucketEncryption(ctx, bucket); err != nil && !hasErrorCode(err, "ServerSideEncryptionConfigurationNotFoundError") {
			return apierrors.NewError(fmt.Errorf("remove bucket encryption failed: %w", err), 502)
		}
	}
	if in.Tags != nil {
		if !caps.BucketTagging {
			return apierrors.NewError(fmt.Errorf("bucket tag management is not supported for provider %s; use the cloud provider console", cc.provider.Key()), 400)
		}
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

// EmptyBucket 递归删除 Bucket 内全部对象、历史版本与删除标记（含目录占位对象），用于删除非空 Bucket 前的清空操作
func (c *controller) EmptyBucket(ctx context.Context, datasourceID int64, bucket string) error {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(bucket) == "" {
		return apierrors.NewError(fmt.Errorf("bucket is required"), 400)
	}
	// 生产者与消费者共用可取消 ctx：删除失败时及时终止列举，否则生产者会阻塞在 channel 发送上。
	produceCtx, cancelProduce := context.WithCancel(ctx)
	defer cancelProduce()

	objectsCh := make(chan minio.ObjectInfo, 1000)
	// listErr 由生产者单 goroutine 写入，主流程在 objectsCh 被 close（建立 happens-before）之后读取
	var listErr error
	go func() {
		defer close(objectsCh)
		for item := range client.ListObjects(produceCtx, bucket, minio.ListObjectsOptions{Recursive: true, WithVersions: true}) {
			if item.Err != nil {
				listErr = item.Err
				return
			}
			select {
			case objectsCh <- minio.ObjectInfo{Key: item.Key, VersionID: item.VersionID}:
			case <-produceCtx.Done():
				return
			}
		}
	}()

	var removeErr error
	for result := range client.RemoveObjects(ctx, bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if result.Err == nil || removeErr != nil {
			continue
		}
		removeErr = fmt.Errorf("delete object(%s) failed: %w", result.ObjectName, result.Err)
		// 记录首个失败后取消列举，但继续把剩余结果读空，保证生产者能退出
		cancelProduce()
	}
	if removeErr != nil {
		return apierrors.NewError(removeErr, 502)
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
		if hasErrorCode(err, "NoSuchCORSConfiguration") {
			return []types.StorageCorsRule{}, nil
		}
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
		// 搜索模式：递归扫描并按关键字过滤，token 为上一个命中的 key；单次最多扫 maxObjectSearchPages 页。
		items := make([]types.StorageObject, 0, pageSize)
		marker := token
		truncated := false
		scannedPages := 0
		for {
			res, err := core.ListObjects(bucket, prefix, marker, "", 1000)
			if err != nil {
				return nil, apierrors.NewError(fmt.Errorf("list storage objects failed: %w", err), 502)
			}
			scannedPages++
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
			if len(res.Contents) == 0 {
				truncated = false
				marker = ""
				break
			}
			marker = res.Contents[len(res.Contents)-1].Key
			if scannedPages >= maxObjectSearchPages {
				truncated = true
				break
			}
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
	dirIndexes := make([]int, 0, len(res.CommonPrefixes))
	for _, cp := range res.CommonPrefixes {
		items = append(items, types.StorageObject{Key: cp.Prefix, IsDir: true})
		dirIndexes = append(dirIndexes, len(items)-1)
	}
	for _, o := range res.Contents {
		if strings.HasSuffix(o.Key, "/") {
			// 忽略模拟目录的占位空对象
			continue
		}
		items = append(items, types.StorageObject{Key: o.Key, Size: o.Size, LastModified: &o.LastModified, ETag: o.ETag})
	}
	// 目录时间放在对象项填充之后并发补齐：不同下标的元素相互独立，可安全并发写
	fillDirectoryTimes(ctx, client, bucket, items, dirIndexes)
	nextToken := ""
	if res.IsTruncated {
		nextToken = res.NextContinuationToken
	}
	return &types.StorageObjectPage{Items: items, NextToken: nextToken, Truncated: res.IsTruncated}, nil
}

// dirTimeConcurrency 目录修改时间补全的并发度：单个目录最坏需要 1 次 Stat + 10 页列举。
const dirTimeConcurrency = 8

// maxObjectSearchPages 对象搜索单次最多扫描的列举页数（每页 1000），避免大桶关键字搜索把接口拖超时
const maxObjectSearchPages = 20

// fillDirectoryTimes 并发补全目录项的修改时间（每个 goroutine 只写自己下标对应的元素）。
// 目录是虚拟前缀：优先取占位对象（key 以 / 结尾）的修改时间，隐式目录再聚合其下最近一次对象修改时间。
func fillDirectoryTimes(ctx context.Context, client *minio.Client, bucket string, items []types.StorageObject, indexes []int) {
	if len(indexes) == 0 {
		return
	}
	core := minio.Core{Client: client}
	sem := make(chan struct{}, dirTimeConcurrency)
	var wg sync.WaitGroup
	for _, index := range indexes {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			key := items[i].Key
			if stat, err := client.StatObject(ctx, bucket, key, minio.StatObjectOptions{}); err == nil && !stat.LastModified.IsZero() {
				lastModified := stat.LastModified
				items[i].LastModified = &lastModified
				return
			}
			if latest, ok := latestObjectTime(core, bucket, key); ok {
				lastModified := latest
				items[i].LastModified = &lastModified
			}
		}(index)
	}
	wg.Wait()
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
	keys = compactDeleteKeys(keys)
	if len(keys) == 0 {
		return apierrors.NewError(fmt.Errorf("object keys are required"), 400)
	}
	// Stream directory expansion into RemoveObjects so a large directory does not become an
	// unbounded in-memory slice.
	produceCtx, cancelProduce := context.WithCancel(ctx)
	defer cancelProduce()
	objectsCh := make(chan minio.ObjectInfo, 1000)
	var listErr error
	go func() {
		defer close(objectsCh)
		send := func(key string) bool {
			select {
			case objectsCh <- minio.ObjectInfo{Key: key}:
				return true
			case <-produceCtx.Done():
				return false
			}
		}
		for _, key := range keys {
			if !strings.HasSuffix(key, "/") {
				if !send(key) {
					return
				}
				continue
			}
			for item := range client.ListObjects(produceCtx, bucket, minio.ListObjectsOptions{Prefix: key, Recursive: true}) {
				if item.Err != nil {
					listErr = item.Err
					return
				}
				if !send(item.Key) {
					return
				}
			}
		}
	}()

	var removeErr error
	for result := range client.RemoveObjects(ctx, bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if result.Err == nil || removeErr != nil {
			continue
		}
		removeErr = fmt.Errorf("delete object(%s) failed: %w", result.ObjectName, result.Err)
		cancelProduce()
	}
	if removeErr != nil {
		return apierrors.NewError(removeErr, 502)
	}
	if listErr != nil {
		return apierrors.NewError(fmt.Errorf("list storage objects failed: %w", listErr), 502)
	}
	return nil
}

// compactDeleteKeys removes duplicate selections and paths covered by a selected directory.
// It does not treat "foo" and "foo/" as interchangeable because they are distinct S3 keys.
func compactDeleteKeys(keys []string) []string {
	unique := make(map[string]struct{}, len(keys))
	for _, rawKey := range keys {
		if key := strings.TrimSpace(rawKey); key != "" {
			unique[key] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(unique))
	for key := range unique {
		ordered = append(ordered, key)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if len(ordered[i]) == len(ordered[j]) {
			return ordered[i] < ordered[j]
		}
		return len(ordered[i]) < len(ordered[j])
	})

	result := make([]string, 0, len(ordered))
	for _, key := range ordered {
		covered := false
		for _, parent := range result {
			if strings.HasSuffix(parent, "/") && strings.HasPrefix(key, parent) {
				covered = true
				break
			}
		}
		if !covered {
			result = append(result, key)
		}
	}
	return result
}

// RenameObject 重命名对象或目录。
// 文件：单 key 复制 + 删除源；目录：S3 里只是前缀，必须把该前缀下所有对象复制到新前缀再删除源。
// 数据经服务端 CopyObject 不经 Pixiu 转发，单次对象数受 maxTransferObjects 约束。
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

	if !strings.HasSuffix(srcKey, "/") {
		stat, err := client.StatObject(ctx, bucket, srcKey, minio.StatObjectOptions{})
		if err != nil {
			return apierrors.NewError(fmt.Errorf("stat storage object failed: %w", err), 502)
		}
		if copyObjectTooLarge(stat.Size) {
			return apierrors.NewError(fmt.Errorf("object %s exceeds the 5GiB CopyObject limit", srcKey), 400)
		}
		if _, err := client.CopyObject(ctx, minio.CopyDestOptions{Bucket: bucket, Object: dstKey}, minio.CopySrcOptions{Bucket: bucket, Object: srcKey}); err != nil {
			return apierrors.NewError(fmt.Errorf("copy object failed: %w", err), 502)
		}
		if err := client.RemoveObject(ctx, bucket, srcKey, minio.RemoveObjectOptions{}); err != nil {
			return apierrors.NewError(fmt.Errorf("remove source object failed: %w", err), 502)
		}
		return nil
	}

	// 目录重命名：前后端都要保证 dstKey 以 / 结尾，否则源目录会「变成」文件
	if !strings.HasSuffix(dstKey, "/") {
		return apierrors.NewError(fmt.Errorf("destination key of a directory must end with /"), 400)
	}
	// 禁止把目录改名到自身子路径下（a/ → a/b/ 会把新对象也纳入同一前缀）
	if strings.HasPrefix(dstKey, srcKey) {
		return apierrors.NewError(fmt.Errorf("cannot rename a directory into itself"), 400)
	}

	// 逐对象复制 + 删除源：中途失败会留下「部分已改名」的状态，错误里带上已处理数量便于手工清理。
	moved := 0
	for obj := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: srcKey, Recursive: true}) {
		if obj.Err != nil {
			return apierrors.NewError(
				fmt.Errorf("list storage objects failed after moving %d objects: %w", moved, obj.Err), 502)
		}
		// 先判后做：达到上限就停止，避免报错里说 max 1000、实际却已经动了 1001 个
		if moved >= maxTransferObjects {
			return apierrors.NewError(
				fmt.Errorf("too many objects in directory (moved %d, max %d)", moved, maxTransferObjects), 400)
		}
		dst := dstKey + strings.TrimPrefix(obj.Key, srcKey)
		if copyObjectTooLarge(obj.Size) {
			return apierrors.NewError(
				fmt.Errorf("object %s exceeds the 5GiB CopyObject limit after moving %d objects", obj.Key, moved), 400)
		}
		if _, err := client.CopyObject(ctx,
			minio.CopyDestOptions{Bucket: bucket, Object: dst},
			minio.CopySrcOptions{Bucket: bucket, Object: obj.Key},
		); err != nil {
			return apierrors.NewError(
				fmt.Errorf("copy object %s failed after moving %d objects: %w", obj.Key, moved, err), 502)
		}
		if err := client.RemoveObject(ctx, bucket, obj.Key, minio.RemoveObjectOptions{}); err != nil {
			return apierrors.NewError(
				fmt.Errorf("object %s moved to %s but removing source failed: %w", obj.Key, dst, err), 502)
		}
		moved++
	}
	return nil
}

// adminClientFor 构造用于用户/策略管理的 MinIO 管理面客户端（会先走一次 clientFor 做鉴权）。
func (c *controller) adminClientFor(ctx context.Context, datasourceID int64) (*madmin.AdminClient, error) {
	_, cc, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	return adminClientFromConfig(cc)
}

// adminClientFromConfig 从已解析的连接配置构造管理面客户端，复用 clientFor 结果。
// 管理面只有 MinIO 兼容端点提供（能力位 AdminAPI）；历史取值 "" 与 "s3" 仍放行，公有云会被拦下。
func adminClientFromConfig(cc clientConfig) (*madmin.AdminClient, error) {
	if !cc.provider.Capabilities().AdminAPI {
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

// formatPolicyNames 把 MinIO 用户策略字段（字符串或数组形式的 JSON）渲染成纯策略名
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
		return nil, apierrors.NewError(fmt.Errorf("list storage users failed (MinIO compatible endpoint required): %w", err), 502)
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
		return apierrors.NewError(fmt.Errorf("create storage user failed (MinIO compatible endpoint required): %w", err), 502)
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
		return apierrors.NewError(fmt.Errorf("delete storage user failed (MinIO compatible endpoint required): %w", err), 502)
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
		return nil, apierrors.NewError(fmt.Errorf("list storage policies failed (MinIO compatible endpoint required): %w", err), 502)
	}
	result := make([]string, 0, len(policies))
	for name := range policies {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}
