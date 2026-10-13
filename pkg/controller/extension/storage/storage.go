package storage

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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
	ListObjects(context.Context, int64, string, string) ([]types.StorageObject, error)
	PresignedGetObject(context.Context, int64, string, string, int) (string, error)
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
		result = append(result, types.StorageObject{Key: item.Key, Size: item.Size, LastModified: item.LastModified, ETag: item.ETag})
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
