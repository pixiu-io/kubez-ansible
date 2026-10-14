/*
Copyright 2021 The Pixiu Authors.

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

package types

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// 资源类型（scope 授权维度）
const (
	ResourceTypePlan         = "plan"
	ResourceTypeCluster      = "cluster"
	ResourceTypeNode         = "node"
	ResourceTypeRunner       = "runner"
	ResourceTypeAgent        = "agent"
	ResourceTypeAccount      = "account" // 用户中心账号
	ResourceTypeDatasource   = "datasource"
	ResourceTypeDistribution = "distribution"
)

type PixiuObjectMeta struct {
	Cluster   string `uri:"cluster" binding:"required"`
	Namespace string `uri:"namespace" binding:"required"`
	Name      string `uri:"name"`
}

// PodResourceMeta Pod 资源定位（cluster + namespace + pod）。
type PodResourceMeta struct {
	Cluster   string `uri:"cluster" binding:"required"`
	Namespace string `uri:"namespace" binding:"required"`
	Pod       string `uri:"pod" binding:"required"`
}

type PixiuMeta struct {
	// pixiu 对象 ID
	Id int64 `json:"id"`
	// Pixiu 对象版本号
	ResourceVersion int64 `json:"resource_version"`
}

type TimeMeta struct {
	// pixiu 对象创建时间
	GmtCreate time.Time `json:"gmt_create"`
	// pixiu 对象修改时间
	GmtModified time.Time `json:"gmt_modified"`
}

type HTTPHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type DatasourceConfig struct {
	Headers  []HTTPHeader          `json:"headers"`
	Log      *LogSourceConfig      `json:"log,omitempty"`
	Alert    *AlertSourceConfig    `json:"alert,omitempty"`
	Redis    *RedisSourceConfig    `json:"redis,omitempty"`
	Nacos    *NacosSourceConfig    `json:"nacos,omitempty"`
	Mysql    *MySQLSourceConfig    `json:"mysql,omitempty"`
	Postgres *PostgresSourceConfig `json:"postgres,omitempty"`
	Storage  *StorageSourceConfig  `json:"storage,omitempty"`
}

// StorageSourceConfig stores provider-specific object storage metadata and credentials.
// Read responses mask sensitive credential fields before returning them to clients.
type StorageSourceConfig struct {
	Provider         string `json:"provider,omitempty"`
	Endpoint         string `json:"endpoint,omitempty"`
	Region           string `json:"region,omitempty"`
	Bucket           string `json:"bucket,omitempty"`
	AccessKeyID      string `json:"access_key_id,omitempty"`
	SecretAccessKey  string `json:"secret_access_key,omitempty"`
	SessionToken     string `json:"session_token,omitempty"`
	SignatureVersion string `json:"signature_version,omitempty"`
	AddressingStyle  string `json:"addressing_style,omitempty"`
}

type StorageBucket struct {
	Name         string    `json:"name"`
	CreationDate time.Time `json:"creation_date,omitempty"`
	Region       string    `json:"region,omitempty"`
}

type StorageObject struct {
	Key          string     `json:"key"`
	Size         int64      `json:"size"`
	LastModified *time.Time `json:"last_modified,omitempty"`
	ETag         string     `json:"etag,omitempty"`
	IsDir        bool       `json:"is_dir,omitempty"`
}

// StorageObjectPage 对象分页列举结果，NextToken 为空表示已列举完毕
// 目录模式下 Token 为 ListObjectsV2 的 continuation token，搜索模式下为marker（上一个命中的 key）
type StorageObjectPage struct {
	Items     []StorageObject `json:"items"`
	NextToken string          `json:"next_token,omitempty"`
	Truncated bool            `json:"truncated"`
}

// StorageObjectVersion 对象历史版本（Bucket 开启多版本后由服务端生成）
// IsDeleteMarker 为 true 表示这是一个「删除标记」版本，对应对象处于已删除状态
type StorageObjectVersion struct {
	Key            string     `json:"key"`
	VersionID      string     `json:"version_id"`
	IsLatest       bool       `json:"is_latest"`
	IsDeleteMarker bool       `json:"is_delete_marker"`
	Size           int64      `json:"size"`
	LastModified   *time.Time `json:"last_modified,omitempty"`
	ETag           string     `json:"etag,omitempty"`
	StorageClass   string     `json:"storage_class,omitempty"`
}

// StorageObjectVersionList 历史版本列举结果
// S3 的 ListObjectVersions 无法从指定版本续读（minio-go 未暴露 version-id-marker），
// 因此这里按上限一次性返回，Truncated 为 true 表示还有更多版本未展开
type StorageObjectVersionList struct {
	Items     []StorageObjectVersion `json:"items"`
	Truncated bool                   `json:"truncated"`
	// VersioningEnabled 表示 Bucket 是否开启了版本控制。
	// 未开启时删除对象不会产生删除标记（即没有回收站），删除是永久的。
	VersioningEnabled bool `json:"versioning_enabled"`
	// VersioningState 为三态：enabled/disabled/unknown。
	// unknown 表示状态查询失败，前端应提示「状态获取失败」而不是断言「未开启」。
	VersioningState string `json:"versioning_state,omitempty"`
}

// StorageMultipartUpload 未完成的分片上传（碎片），占用存储空间但不属于任何对象
type StorageMultipartUpload struct {
	Key       string     `json:"key"`
	UploadID  string     `json:"upload_id"`
	Initiated *time.Time `json:"initiated,omitempty"`
	Size      int64      `json:"size"`
}

// StorageMultipartUploadList 碎片列举结果，Truncated 为 true 表示超出上限未全部展开
type StorageMultipartUploadList struct {
	Items     []StorageMultipartUpload `json:"items"`
	TotalSize int64                    `json:"total_size"`
	Truncated bool                     `json:"truncated"`
}

// StorageAbortUploadResult 碎片清理结果明细。部分失败也返回 200，仅参数/连接类错误才走 error。
type StorageAbortUploadResult struct {
	Succeeded  int      `json:"succeeded"`
	Failed     int      `json:"failed"`
	FailedKeys []string `json:"failed_keys,omitempty"`
}

// ---------- 大文件分片上传（S3 Multipart Upload）----------

// StorageMultipartInit 初始化分片上传请求
type StorageMultipartInit struct {
	Bucket       string `json:"bucket" binding:"required"`
	Key          string `json:"key" binding:"required"`
	ContentType  string `json:"content_type,omitempty"`
	StorageClass string `json:"storage_class,omitempty"`
}

// StorageMultipartSession 初始化结果。PartSize 由服务端下发，前端按此切分；
// S3 限制单次分片上传最多 10000 片，且单片不得小于 5MiB（最后一片除外）。
type StorageMultipartSession struct {
	UploadID string `json:"upload_id"`
	PartSize int64  `json:"part_size"`
	MaxParts int    `json:"max_parts"`
}

// StorageMultipartPart 已上传的分片（用于断点续传与完成校验）
type StorageMultipartPart struct {
	PartNumber   int        `json:"part_number"`
	ETag         string     `json:"etag"`
	Size         int64      `json:"size"`
	LastModified *time.Time `json:"last_modified,omitempty"`
}

// StorageMultipartCompletePart 完成分片上传时提交的分片清单
type StorageMultipartCompletePart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
}

// StorageMultipartComplete 完成分片上传请求
type StorageMultipartComplete struct {
	Bucket   string                         `json:"bucket" binding:"required"`
	Key      string                         `json:"key" binding:"required"`
	UploadID string                         `json:"upload_id" binding:"required"`
	Parts    []StorageMultipartCompletePart `json:"parts" binding:"required"`
}

// StorageMultipartTarget 分片上传的目标（完成/中止/列举分片共用）
type StorageMultipartTarget struct {
	Bucket   string `json:"bucket" binding:"required"`
	Key      string `json:"key" binding:"required"`
	UploadID string `json:"upload_id" binding:"required"`
}

// StorageMultipartResult 完成分片上传后的对象信息
type StorageMultipartResult struct {
	Key  string `json:"key"`
	ETag string `json:"etag"`
	Size int64  `json:"size"`
}

// ---------- Bucket Policy 原始策略编辑 ----------

// StorageBucketPolicy Bucket 策略原始 JSON。
// 相较 StorageBucketConfig.ACL 的三档归类，这里是完整策略文本，
// 支持精细到前缀/动作的授权（Exists 为 false 表示当前无策略）。
type StorageBucketPolicy struct {
	Policy string `json:"policy"`
	Exists bool   `json:"exists"`
	ACL    string `json:"acl,omitempty"`
}

// StorageBucketPolicyUpdate 策略更新请求；Policy 为空串表示删除策略
type StorageBucketPolicyUpdate struct {
	Policy string `json:"policy"`
}

// ---------- 跨桶复制 / 移动 ----------

// StorageTransferRequest 跨桶复制或移动对象请求。
// Keys 中的目录（以 / 结尾）会被递归展开；Move 为 true 时复制成功后删除源对象。
type StorageTransferRequest struct {
	Bucket    string   `json:"bucket" binding:"required"`
	Keys      []string `json:"keys" binding:"required"`
	DstBucket string   `json:"dst_bucket" binding:"required"`
	DstPrefix string   `json:"dst_prefix"`
	Move      bool     `json:"move"`
}

// StorageTransferResult 传输结果明细：部分失败也返回 200，由前端提示成功/失败数
type StorageTransferResult struct {
	Succeeded   int      `json:"succeeded"`
	Failed      int      `json:"failed"`
	FailedItems []string `json:"failed_items,omitempty"`
	Total       int      `json:"total"`
}

// StorageObjectTag 对象标签键值对
type StorageObjectTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// StorageObjectDetail 对象详情与元数据（以 StatObject 为准，跨厂商通用）
type StorageObjectDetail struct {
	Key             string             `json:"key"`
	VersionID       string             `json:"version_id,omitempty"`
	Size            int64              `json:"size"`
	ETag            string             `json:"etag,omitempty"`
	ContentType     string             `json:"content_type,omitempty"`
	ContentEncoding string             `json:"content_encoding,omitempty"`
	CacheControl    string             `json:"cache_control,omitempty"`
	LastModified    *time.Time         `json:"last_modified,omitempty"`
	StorageClass    string             `json:"storage_class,omitempty"`
	ACL             string             `json:"acl,omitempty"`
	UserMetadata    map[string]string  `json:"user_metadata,omitempty"`
	Tags            []StorageObjectTag `json:"tags,omitempty"`
}

// StorageObjectMetaUpdate 对象元数据更新请求，仅更新显式传入的字段
// 未传入的字段保持对象当前值（后端会先读取现状再合并写回）
type StorageObjectMetaUpdate struct {
	ContentType     *string            `json:"content_type,omitempty"`
	ContentEncoding *string            `json:"content_encoding,omitempty"`
	CacheControl    *string            `json:"cache_control,omitempty"`
	UserMetadata    *map[string]string `json:"user_metadata,omitempty"`
}

// StorageBucketConfig Bucket 配置（对齐阿里云 OSS / 腾讯云 COS / 华为云 OBS 的通用配置项）
// ACL: private / public-read / public-read-write；Versioning: Enabled / Suspended，空表示未开启
type StorageBucketConfig struct {
	ACL        string            `json:"acl,omitempty"`
	Versioning string            `json:"versioning,omitempty"`
	Encryption bool              `json:"encryption"`
	Tags       map[string]string `json:"tags,omitempty"`
}

// StorageBucketConfigUpdate 仅更新显式传入的字段
type StorageBucketConfigUpdate struct {
	ACL        *string           `json:"acl,omitempty"`
	Versioning *string           `json:"versioning,omitempty"`
	Encryption *bool             `json:"encryption,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
}

// StorageBucketStats 表示当前 Bucket 的对象数量与总对象大小，不包含历史版本和删除标记。
type StorageBucketStats struct {
	Objects uint64 `json:"objects"`
	Bytes   uint64 `json:"bytes"`
}

// StorageLifecycleRule Bucket 生命周期规则：匹配前缀的对象过期自动删除，并清理未完成的分片上传
// Status: Enabled / Disabled
type StorageLifecycleRule struct {
	ID                 string `json:"id,omitempty"`
	Status             string `json:"status"`
	Prefix             string `json:"prefix,omitempty"`
	ExpirationDays     int    `json:"expiration_days,omitempty"`
	AbortMultipartDays int    `json:"abort_multipart_days,omitempty"`
}

// StorageCorsRule Bucket 跨域资源共享规则
type StorageCorsRule struct {
	ID             string   `json:"id,omitempty"`
	AllowedOrigins []string `json:"allowed_origins"`
	AllowedMethods []string `json:"allowed_methods"`
	AllowedHeaders []string `json:"allowed_headers,omitempty"`
	ExposeHeaders  []string `json:"expose_headers,omitempty"`
	MaxAgeSeconds  int      `json:"max_age_seconds,omitempty"`
}

type StorageUser struct {
	AccessKey string `json:"access_key"`
	Status    string `json:"status,omitempty"`
	Policy    string `json:"policy,omitempty"`
}

type StorageUserCreate struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Policy    string `json:"policy,omitempty"`
}

type StoragePing struct {
	Connected bool   `json:"connected"`
	Provider  string `json:"provider,omitempty"`
}

// StorageCapabilities 厂商在控制面上的能力子集，供前端决定功能入口是否展示。
type StorageCapabilities struct {
	BucketPolicy  bool `json:"bucket_policy"`
	SSEEncryption bool `json:"sse_s3_encryption"`
	BucketTagging bool `json:"bucket_tags"`
	AdminAPI      bool `json:"user_management"`
}

// StorageProviderInfo 厂商标识及其控制面能力。
type StorageProviderInfo struct {
	Key          string              `json:"key"`
	Capabilities StorageCapabilities `json:"capabilities"`
}

// StoragePresignedObject 临时下载地址。
// Expires 是后端夹取后的**生效**有效期（秒），不是请求值——请求 0 或超过上限时会被改成默认 900。
type StoragePresignedObject struct {
	URL     string `json:"url"`
	Expires int    `json:"expires"`
}

// NacosSourceConfig Nacos 数据源附加配置
// Version 取值 v2/v3：v2 使用 Nacos 2.x（v1 OpenAPI），v3 使用 Nacos 3.x（v3 Console API）
type NacosSourceConfig struct {
	Version string `json:"version,omitempty"`
}

type LogSourceConfig struct {
	URL      string `json:"url,omitempty"`
	UserName string `json:"user_name,omitempty"`
	Password string `json:"password,omitempty"`
}

type AlertSourceConfig struct {
	URL string `json:"url,omitempty"`

	UserName string `json:"user_name,omitempty"`
	Password string `json:"password,omitempty"`
}

// Redis 部署模式
const (
	RedisModeStandalone = "standalone" // 单机
	RedisModeSentinel   = "sentinel"   // 哨兵
	RedisModeCluster    = "cluster"    // 集群
)

// RedisSourceConfig Redis 数据源连接配置（仅外部直连）
type RedisSourceConfig struct {
	Mode             string   `json:"mode,omitempty"`              // standalone/sentinel/cluster，空视为 standalone（兼容存量数据）
	Address          string   `json:"address,omitempty"`           // standalone 连接地址 host:port
	Addresses        []string `json:"addresses,omitempty"`         // sentinel/cluster 节点地址列表
	MasterName       string   `json:"master_name,omitempty"`       // sentinel master 名称
	Password         string   `json:"password,omitempty"`          // 数据节点密码
	SentinelPassword string   `json:"sentinel_password,omitempty"` // 哨兵节点密码（仅 sentinel）
	DB               int      `json:"db,omitempty"`                // DB 编号（cluster 固定 0）
}

// NormalizeMode 归一化部署模式，未知/空值按 standalone 处理
func (r *RedisSourceConfig) NormalizeMode() string {
	switch r.Mode {
	case RedisModeSentinel:
		return RedisModeSentinel
	case RedisModeCluster:
		return RedisModeCluster
	default:
		return RedisModeStandalone
	}
}

// DisplayAddress 用于日志/探测结果展示的连接摘要
func (r *RedisSourceConfig) DisplayAddress() string {
	switch r.NormalizeMode() {
	case RedisModeSentinel:
		return fmt.Sprintf("sentinel://%s@%s", r.MasterName, strings.Join(r.Addresses, ","))
	case RedisModeCluster:
		return "cluster://" + strings.Join(r.Addresses, ",")
	default:
		return r.Address
	}
}

// MySQLSourceConfig MySQL 数据源连接配置（仅外部直连）
type MySQLSourceConfig struct {
	Host     string `json:"host,omitempty"`      // 连接地址
	Port     int    `json:"port,omitempty"`      // 端口，缺省 3306
	UserName string `json:"user_name,omitempty"` // 用户名
	Password string `json:"password,omitempty"`  // 密码
	Database string `json:"database,omitempty"`  // 默认库（可空，表示实例级连接）
	Charset  string `json:"charset,omitempty"`   // 连接字符集，缺省 utf8mb4
	Params   string `json:"params,omitempty"`    // 附加 DSN 参数（key=value&...），服务端白名单校验
	Timeout  int    `json:"timeout,omitempty"`   // 连接超时秒数，缺省 5
}

// PostgresSourceConfig PostgreSQL 数据源连接配置（仅外部直连）
type PostgresSourceConfig struct {
	Host            string `json:"host,omitempty"`
	Port            int    `json:"port,omitempty"`
	UserName        string `json:"user_name,omitempty"`
	Password        string `json:"password,omitempty"`
	Database        string `json:"database,omitempty"`
	SSLMode         string `json:"ssl_mode,omitempty"`
	ConnectTimeout  int    `json:"connect_timeout,omitempty"`
	ApplicationName string `json:"application_name,omitempty"`
}

func (p *PostgresSourceConfig) DisplayAddress() string { return fmt.Sprintf("%s:%d", p.Host, p.Port) }
func (p *PostgresSourceConfig) NormalizePort() int {
	if p.Port <= 0 || p.Port > 65535 {
		return 5432
	}
	return p.Port
}

// DisplayAddress 用于日志/探测结果展示的连接摘要（脱敏，不含密码）
func (m *MySQLSourceConfig) DisplayAddress() string {
	return fmt.Sprintf("%s:%d", m.Host, m.Port)
}

// NormalizePort 归一化端口，非法值回退默认 3306
func (m *MySQLSourceConfig) NormalizePort() int {
	if m.Port <= 0 || m.Port > 65535 {
		return 3306
	}
	return m.Port
}

type KubeNode struct {
	// Total 集群节点总数（拨测 Limit=1 + RemainingItemCount 回写）
	Total int `json:"total"`
}

type Cluster struct {
	PixiuMeta `json:",inline"`

	Name      string              `json:"name"`
	AliasName string              `json:"alias_name"`
	Status    model.ClusterStatus `json:"status"` // 0: 运行中 1: 部署中 2: 等待部署 3: 部署失败 4: 集群失联，API不可用

	UserId int64 `json:"user_id"`

	// 0: 标准集群 1: 自建集群
	ClusterType model.ClusterType `json:"cluster_type"`
	PlanId      int64             `json:"plan_id"` // 自建集群关联的 PlanId，如果是自建的集群，planId 不为 0

	PermissionId int64 `json:"permission_id"`

	// kubernetes 集群的版本和状态
	KubernetesVersion string   `json:"kubernetes_version"`
	Nodes             KubeNode `json:"nodes"`

	// 集群删除保护，开启集群删除保护时不允许删除集群
	// 0: 关闭集群删除保护 1: 开启集群删除保护
	Protected bool `json:"protected"`

	// 连接模式：0 直连 1 Agent 反向隧道
	ConnectMode model.ConnectMode `json:"connect_mode"`
	// Agent 是否在线（隧道模式）
	AgentConnected bool `json:"agent_connected,omitempty"`
	// Agent 建连 token（仅创建/详情时返回，用于安装 Agent）
	AgentToken string `json:"agent_token,omitempty"`

	// k8s kubeConfig base64 字段
	KubeConfig string `json:"kube_config,omitempty"`

	// 集群用途描述，可以为空
	Description string `json:"description"`

	// 集群连通性探测状态（Ready condition 语义），由 syncer 定期维护
	ProbeStatus   model.ClusterProbeStatus `json:"probe_status"`
	ProbeReason   string                   `json:"probe_reason"`
	ProbeMessage  string                   `json:"probe_message"`
	LastProbeTime *time.Time               `json:"last_probe_time,omitempty"`

	KubernetesMeta `json:",inline"`
	TimeMeta       `json:",inline"`
}

type Datasource struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	UserId      int64                   `json:"user_id"`
	ClusterName string                  `json:"cluster_name"`
	Name        string                  `json:"name"`
	Type        model.DatasourceType    `json:"type"`
	SubType     model.DatasourceSubType `json:"sub_type"`
	Config      DatasourceConfig        `json:"config"`
	IsDefault   bool                    `json:"is_default"`
	External    bool                    `json:"external"`
	Description string                  `json:"description"`
}

// Email 系统邮件 SMTP 配置响应。
// PasswordSet 表示是否已配置密码（明文密码不回显）。
type Email struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name        string `json:"name"`
	SmtpHost    string `json:"smtp_host"`
	SmtpPort    int    `json:"smtp_port"`
	Username    string `json:"username"`
	PasswordSet bool   `json:"password_set"`
	FromEmail   string `json:"from_email"`
	FromName    string `json:"from_name"`
	Encryption  string `json:"encryption"`
	Enabled     bool   `json:"enabled"`
	IsDefault   bool   `json:"is_default"`
	Description string `json:"description"`
	CreatedBy   int64  `json:"created_by"`
}

type AIProvider struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name        string `json:"name"`
	BaseURL     string `json:"base_url"`
	Protocol    string `json:"protocol"`
	Description string `json:"description"`
	MaxTokens   int    `json:"max_tokens"`
	Builtin     bool   `json:"builtin"`
}

type AIAccount struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name       string      `json:"name"`
	APIKey     string      `json:"api_key"`
	Model      string      `json:"model"`
	ProviderId int64       `json:"provider_id"`
	UserId     int64       `json:"user_id"`
	Provider   *AIProvider `json:"provider,omitempty"`
}

type Conversation struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	ProviderId         int64  `json:"provider_id"`
	Provider           string `json:"provider"`
	Model              string `json:"model"`
	Title              string `json:"title"`
	PreviousResponseId string `json:"previous_response_id"`
	History            string `json:"history"`
}

type Message struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	RequestId       string `json:"request_id"`
	ProviderId      int64  `json:"provider_id"`
	ConversationId  int64  `json:"conversation_id"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	ResponseId      string `json:"response_id"`
	InputText       string `json:"input_text"`
	OutputText      string `json:"output_text"`
	Success         bool   `json:"success"`
	ErrorMessage    string `json:"error_message"`
	Duration        int64  `json:"duration"`
	InputTokens     int64  `json:"input_tokens"`
	OutputTokens    int64  `json:"output_tokens"`
	TotalTokens     int64  `json:"total_tokens"`
	CachedTokens    int64  `json:"cached_tokens"`
	ReasoningTokens int64  `json:"reasoning_tokens"`
}

type AIRespondResponse struct {
	ConversationId int64       `json:"conversation_id"`
	ResponseId     string      `json:"response_id"`
	Text           string      `json:"text"`
	Model          string      `json:"model"`
	Raw            interface{} `json:"raw,omitempty"`
}

type AIStreamEvent struct {
	Type           string      `json:"type"`
	Stage          string      `json:"stage,omitempty"`
	Message        string      `json:"message,omitempty"`
	Delta          string      `json:"delta,omitempty"`
	Text           string      `json:"text,omitempty"`
	Model          string      `json:"model,omitempty"`
	ToolName       string      `json:"tool_name,omitempty"`
	ToolCallId     string      `json:"tool_call_id,omitempty"`
	ToolArgs       string      `json:"tool_args,omitempty"`
	ToolOutput     string      `json:"tool_output,omitempty"`
	ConversationId int64       `json:"conversation_id,omitempty"`
	ResponseId     string      `json:"response_id,omitempty"`
	Raw            interface{} `json:"raw,omitempty"`
}

// KubernetesMeta 记录 kubernetes 集群的数据
type KubernetesMeta struct {
	// 集群的版本
	KubernetesVersion string `json:"kubernetes_version,omitempty"`
	// 节点数量
	Nodes int `json:"nodes"`
	// The memory and cpu usage
	Resources Resources `json:"resources"`
}

// Resources kubernetes 的资源信息
// The memory and cpu usage
type Resources struct {
	Cpu    string `json:"cpu"`
	Memory string `json:"memory"`
}

type User struct {
	PixiuMeta `json:",inline"`

	Name        string           `json:"name"`                                 // 用户名称
	Password    string           `json:"password" binding:"required,password"` // 用户密码
	Status      model.UserStatus `json:"status"`                               // 用户状态标识
	Role        model.UserLevel  `json:"role"`                                 // 用户角色，目前只实现管理员，0: 普通用户 1: 管理员 2: 超级管理员
	Email       string           `json:"email"`                                // 用户注册邮件
	Phone       string           `json:"phone"`                                // 用户手机号
	Description string           `json:"description"`                          // 用户描述信息

	TimeMeta `json:",inline"`
}

type Tenant struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name        string `json:"name"`        // 用户名称
	Description string `json:"description"` // 用户描述信息
}

type Role struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	TenantId    int64  `json:"tenant_id"` // 0 表示系统全局角色
	Name        string `json:"name"`
	Description string `json:"description"`
}

type APIResource struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Method      string `json:"method"`
	Path        string `json:"path"`
	Group       string `json:"group"`
	Description string `json:"description"`
}

type RoleAPIsResponse struct {
	Associated   []APIResource `json:"associated"`
	Unassociated []APIResource `json:"unassociated"`
}

// RoleAPIScope 角色对 pixiu 自有资源的授权
type RoleAPIScope struct {
	APIId        int64  `json:"api_id"`
	ResourceType string `json:"resource_type"`
	ResourceId   int64  `json:"resource_id"`
}

// RoleAPIScopesResponse 角色 pixiu 资源权限
type RoleAPIScopesResponse struct {
	Scopes []RoleAPIScope `json:"scopes"`
	APIs   []APIResource  `json:"apis"`
}

// MenuResource 菜单目录项（与 APIResource 对称，供角色菜单授权下发）
type MenuResource struct {
	Code         string   `json:"code"`
	ParentCode   string   `json:"parent_code,omitempty"`
	Title        string   `json:"title"`
	Path         string   `json:"path,omitempty"`
	Kind         string   `json:"kind"`
	Public       bool     `json:"public,omitempty"`
	AdminOnly    bool     `json:"admin_only,omitempty"`
	RequiredAPIs []string `json:"required_apis,omitempty"`
}

// RoleMenusResponse 角色菜单权限
type RoleMenusResponse struct {
	Associated []string       `json:"associated"` // 当前生效菜单码
	Catalog    []MenuResource `json:"catalog"`    // 全量目录
	Derived    bool           `json:"derived"`    // true 表示尚无显式绑定，当前为 API 推导结果
}

// CurrentUserPermissionsResponse 当前登录用户可用权限（供前端控制显示）
// 三层模型：menus（菜单）/ buttons（操作 API）/ scopes（数据）
type CurrentUserPermissionsResponse struct {
	Role    model.UserLevel `json:"role"`
	IsRoot  bool            `json:"is_root"`
	APIs    []APIResource   `json:"apis"`
	Scopes  []RoleAPIScope  `json:"scopes"`
	Buttons []string        `json:"buttons"` // METHOD:path，供前端 hasAuth / ValidAccess 对齐
	Menus   []string        `json:"menus"`   // 菜单业务码，供侧栏与路由可见性
}

type Plan struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name              string             `json:"name"` // 用户名称
	Step              model.TaskStatus   `json:"step"`
	Description       string             `json:"description"`        // 用户描述信息
	KubernetesVersion string             `json:"kubernetes_version"` // k8s 版本
	NodeCount         int                `json:"node_count"`         // 节点总数
	ExecMode          model.PlanExecMode `json:"exec_mode"`
	DeployAgentId     int64              `json:"deploy_agent_id,omitempty"`

	Config PlanConfig `json:"config"`
	Nodes  []PlanNode `json:"nodes"`
}

type PlanNode struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name   string       `json:"name"` // required
	UserId int64        `json:"user_id,omitempty"`
	PlanId int64        `json:"plan_id,omitempty"`
	Role   []string     `json:"role"` // k8s 节点的角色，master 和 node
	CRI    model.CRI    `json:"cri"`
	Ip     string       `json:"ip"`
	Auth   PlanNodeAuth `json:"auth,omitempty"`
}

type Audit struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Ip                string                     `json:"ip"`
	Action            string                     `json:"action"`             // 操作动作
	Status            model.AuditOperationStatus `json:"status"`             // 操作状态
	Operator          string                     `json:"operator"`           // 操作人
	Path              string                     `json:"path"`               // 操作路径
	ObjectType        model.ObjectType           `json:"resource_type"`      // 资源类型
	Duration          int64                      `json:"duration"`           // 请求耗时 ms
	ResponseCode      int                        `json:"response_code"`      // HTTP 响应码
	Cluster           string                     `json:"cluster"`            // K8s 集群名
	ResourceName      string                     `json:"resource_name"`      // 资源名称
	ResourceNamespace string                     `json:"resource_namespace"` // 资源命名空间
}

type AuthType string

const (
	NoneAuth     AuthType = "none"     // 已开启密码
	KeyAuth      AuthType = "key"      // 密钥
	PasswordAuth AuthType = "password" // 密码
)

const (
	// PodFileMaxBytes 单次 pod 文件传输（上传/下载）负载大小上限。
	PodFileMaxBytes int64 = 100 * 1024 * 1024 // 100MiB
)

const (
	defaultExpiration        = 365 * 24 * time.Hour
	defaultExpirationSeconds = int64(defaultExpiration / time.Second)
	defaultNamespace         = "pixiu-system"
)

type PlanNodeAuth struct {
	Type     AuthType      `json:"type"` // 节点认证模式，支持 key 和 password
	Port     int           `json:"port,omitempty"`
	Key      *KeySpec      `json:"key,omitempty"`
	Password *PasswordSpec `json:"password,omitempty"`
}

const DefaultSSHPort = 22

const (
	PixiuViewClusterRole = "pixiu-view"
	ClusterAdminRole     = "cluster-admin"
)

// SSHPort returns the configured SSH port, falling back to the standard port
// for old node records and invalid values.
func (a PlanNodeAuth) SSHPort() int {
	if a.Port < 1 || a.Port > 65535 {
		return DefaultSSHPort
	}
	return a.Port
}

type PlanTask struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Name    string           `json:"name"`
	PlanId  int64            `json:"plan_id" binding:"required"`
	Action  string           `json:"action"`
	Status  model.TaskStatus `json:"status"`
	Message string           `json:"message"`
}

type KeySpec struct {
	Data string `json:"data,omitempty"`
	File string `json:"-"`
}

type PasswordSpec struct {
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}

type PlanConfig struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	PlanId  int64  `json:"plan_id,omitempty"` // required
	Region  string `json:"region"`
	OSImage string `json:"os_image"` // 操作系统

	Kubernetes KubernetesSpec `json:"kubernetes"`
	Network    NetworkSpec    `json:"network"`
	Runtime    RuntimeSpec    `json:"runtime"`
	Component  ComponentSpec  `json:"component"` // 支持的扩展组件配置

}

// Distribution 部署支持的操作系统发行版
type Distribution struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	Family string `json:"family"`
	Name   string `json:"name"`
	Runner string `json:"runner"`
}

// TimeSpec 通用时间规格
type TimeSpec struct {
	GmtCreate   interface{} `json:"gmt_create,omitempty"`
	GmtModified interface{} `json:"gmt_modified,omitempty"`
}

type KubeObject struct {
	lock sync.RWMutex

	ReplicaSets []appv1.ReplicaSet
	Pods        []v1.Pod
}

// WebShellOptions ws API 参数定义
type WebShellOptions struct {
	Cluster     string   `form:"cluster"`
	Namespace   string   `form:"namespace"`
	Pod         string   `form:"pod"`
	Container   string   `form:"container"`
	Command     string   `form:"command"`
	CommandArgs []string `form:"-"`
}

// TerminalMessage 定义了终端和容器 shell 交互内容的格式 Operation 是操作类型
// Data 是具体数据内容 Rows和Cols 可以理解为终端的行数和列数，也就是宽、高
type TerminalMessage struct {
	Operation string `json:"operation"`
	Data      string `json:"data"`
	Rows      uint16 `json:"rows"`
	Cols      uint16 `json:"cols"`
}

// TerminalSession 定义 TerminalSession 结构体，实现 PtyHandler 接口
// wsConn 是 websocket 连接
// sizeChan 用来定义终端输入和输出的宽和高
// doneChan 用于标记退出终端
type TerminalSession struct {
	wsConn   *websocket.Conn
	sizeChan chan remotecommand.TerminalSize
	doneChan chan struct{}
}

type Turn struct {
	StdinPipe io.WriteCloser
	Session   *ssh.Session
	WsConn    *websocket.Conn
}

// ListOptions is the query options to a standard REST list call.
type ListOptions struct {
	UserId   int64 `form:"user_id" json:"user_id"` // 用户ID，不接受用户传，直接从会话里获取
	UserRole model.UserLevel

	CustomMeta  `form:",inline" json:",inline"`
	PageRequest `form:",inline" json:",inline"` // 分页请求属性
	QueryOption `form:",inline" json:",inline"` // 搜索内容
}

type CustomMeta struct {
	Status *int   `form:"status" json:"status"`
	Step   string `form:"step" json:"step"` // plan 查询的时候需要 状态过滤，不传则不过滤

	ClusterName    string                  `form:"cluster_name" json:"cluster_name"`
	DatasourceType *model.DatasourceType   `form:"datasource_type" json:"datasource_type"`
	SubType        model.DatasourceSubType `form:"sub_type" json:"sub_type"`
	Provider       string                  `form:"provider" json:"provider"`
	ProviderId     int64                   `form:"provider_id" json:"provider_id"`
	Enabled        *bool                   `form:"enabled" json:"enabled"`
	ConversationId int64                   `form:"conversation_id" json:"conversation_id"`

	// alert
	RuleId      int64                    `form:"rule_id" json:"rule_id"`
	EventId     int64                    `form:"event_id" json:"event_id"`
	ClusterId   int64                    `form:"cluster_id" json:"cluster_id"`
	Severity    model.AlertSeverity      `form:"severity" json:"severity"`
	ChannelType model.AlertNotifyChannel `form:"channel_type" json:"channel_type"`

	// user
	UserPhone string `form:"userPhone" json:"userPhone"`
	UserEmail string `form:"userEmail" json:"userEmail"`

	// role
	TenantId *int64 `form:"tenant_id" json:"tenant_id"`

	// api resource
	Method       string `form:"method" json:"method"`
	PathSelector string `form:"pathSelector" json:"pathSelector"`
	Group        string `form:"group" json:"group"`

	// distribution
	Family string `form:"family" json:"family"`

	// audit
	Operator   string `form:"operator" json:"operator"`
	Action     string `form:"action" json:"action"`
	ObjectType string `form:"object_type" json:"object_type"`
	StartTime  string `form:"start_time" json:"start_time"`
	EndTime    string `form:"end_time" json:"end_time"`

	// agent
	AgentStatus *model.AgentStatus `form:"agent_status" json:"agent_status"`

	// node
	PlanId *int64 `form:"plan_id" json:"plan_id"`
}

func (o *ListOptions) SetDefaultPageOption() {
	// 初始化分页属性
	if o.Page <= 0 {
		o.Page = 1
	}
	if o.Limit <= 0 {
		o.Limit = 10
	}
	if o.Limit > 100 {
		o.Limit = 100
	}
}

// SetUserOption 应用当前用户到查询条件：仅超级管理员（RoleRoot）保留客户端 UserId（0=全部）；
// 其他角色强制为当前用户。
func (o *ListOptions) SetUserOption(user *model.User) {
	if user.Role != model.RoleRoot {
		o.UserId = user.Id
	}
}

type EventOptions struct {
	Uid        string `form:"uid"`
	Namespace  string `form:"namespace"`
	Name       string `form:"name"`
	Kind       string `form:"kind"`
	Namespaced bool   `form:"namespaced"`
	Limit      int64  `form:"limit"`
}

type PodLogOptions struct {
	Container  string `form:"container"`
	TailLines  int64  `form:"tailLines"`
	SinceTime  string `form:"sinceTime"`
	Timestamps bool   `form:"timestamps"`
}

// PodFileOptions query for pod file browse APIs.
type PodFileOptions struct {
	Container string `form:"container" binding:"required"`
	Path      string `form:"path"`
}

// PodFileEntry is one filesystem entry inside a pod container.
type PodFileEntry struct {
	Name    string `json:"name"`
	Type    string `json:"type"` // dir | file | link | other
	Size    int64  `json:"size"`
	ModTime string `json:"modTime,omitempty"`
	Mode    string `json:"mode,omitempty"`
	Uid     string `json:"uid,omitempty"`
	Gid     string `json:"gid,omitempty"`
}

// PodFileListResult is the list response for pod file browse.
type PodFileListResult struct {
	Path  string         `json:"path"`
	Items []PodFileEntry `json:"items"`
}

type KubernetesSpec struct {
	EnablePublicIp    bool   `json:"enable_public_ip"`
	ApiServer         string `json:"api_server"`
	ApiPort           string `json:"api_port"`
	KubernetesVersion string `json:"kubernetes_version"`
	EnableHA          bool   `json:"enable_ha"`
	Register          bool   `json:"register"`
	ImageRepository   string `json:"image_repository,omitempty"` // kubernetes 镜像仓库地址
	SetHostname       bool   `json:"set_hostname"`               // 自动修改k8s节点名称, Rocky 系统不生效
	Protect           bool   `json:"protect"`                    // 开启集群保护，防止误删除
	ChangeSelinux     bool   `json:"change_selinux"`             // 关闭 Selinux
}

type NetworkSpec struct {
	NetworkInterface string `json:"network_interface"`                                  // 网口，默认 eth0
	Cni              string `json:"cni" binding:"required,oneof=calico flannel cilium"` // 容器网络插件
	PodNetwork       string `json:"pod_network"`
	ServiceNetwork   string `json:"service_network"`
	KubeProxy        string `json:"kube_proxy"`
}

type RuntimeSpec struct {
	Runtime string `json:"runtime"`
	DataDir string `json:"data_dir"` // 自定义容器运行时数据存放目录
}

type ComponentSpec struct {
	Helm         *Helm         `json:"helm,omitempty"` // 忽略，则使用默认值
	Prometheus   *Prometheus   `json:"prometheus,omitempty"`
	Grafana      *Grafana      `json:"grafana,omitempty"`
	Haproxy      *Haproxy      `json:"haproxy,omitempty"`
	MetricServer *MetricServer `json:"metric_server,omitempty"`
	IngressNginx *IngressNginx `json:"ingress_nginx,omitempty"`
	NFS          *NFS          `json:"nfs,omitempty"`

	CustomRepo        *CustomRepo        `json:"custom_repo,omitempty"`
	CustomConfigs     []CustomConfigItem `json:"custom_configs,omitempty"`     // 用户自定义 globals 键值对
	CertificatePeriod *CertificatePeriod `json:"certificate_period,omitempty"` // 证书有效期
}

// CustomConfigItem 部署计划自定义配置项，渲染到 globals.yml「# 组件默认开关」上方
type CustomConfigItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Helm struct {
	Enable      bool   `json:"enable"`
	HelmRelease string `json:"helm_release"`
}

type CustomRepo struct {
	Enable  bool   `json:"enable"`
	Content string `json:"content"`
}

type CertificatePeriod struct {
	Enable bool `json:"enable"`

	CertificateValidityPeriod   string `json:"certificate_validity_period"`    // 证书有效期 默认 1你那
	CaCertificateValidityPeriod string `json:"ca_certificate_validity_period"` // 根证书有效期 默认 10年
}

type NFS struct {
	Enable bool `json:"enable"`

	StorageClassName string `json:"storage_class_name"` // 指定 nfs 存储名称
	StorageDataDir   string `json:"storage_data_dir"`   // 指定 nfs server 存储地址
}

type MetricServer struct {
	Enable bool `json:"enable"`
}

type IngressNginx struct {
	Enable bool `json:"enable"`
}

type Prometheus struct {
	EnablePrometheus string `json:"enable_prometheus"`
	Enable           bool   `json:"enable"`
}

type Grafana struct {
	Enable               bool   `json:"enable"`
	GrafanaAdminUser     string `json:"grafana_admin_user"`
	GrafanaAdminPassword string `json:"grafana_admin_password"`
}

// Haproxy Options
// This configuration is usually enabled when self-created VMs require high availability.
type Haproxy struct {
	Enable                    bool   `json:"enable"`                       // Enable haproxy and keepalived,
	KeepalivedVirtualRouterId string `json:"keepalived_virtual_router_id"` // Arbitrary unique number from 0..255
}

// CreatePermissionRequest 创建 scoped kubeconfig 的请求参数
type CreatePermissionRequest struct {
	ClusterId         int64  `json:"cluster_id" binding:"required"` // 授权k8s的集群ID，主集群
	UserId            int64  `json:"user_id"`
	Name              string `json:"name" binding:"required"`
	ExpirationSeconds int64  `json:"expiration_seconds"` // 默认 1 年
	Description       string `json:"description"`

	PType int                 `json:"p_type"` // 0 只读，1 自定义，2 管理员
	Rules []rbacv1.PolicyRule `json:"rules"`  // p_type=1 时使用

	// SA/RBAC 对象名由服务端按用户生成，忽略客户端传入。
	SAName          string `json:"-"`
	SANamespace     string `json:"-"`
	ClusterRoleName string `json:"-"`
	RoleBindingName string `json:"-"`

	TargetNamespaces []string `json:"target_namespaces"`
}

// UpdatePermissionRequest 更新权限
type UpdatePermissionRequest struct {
	PixiuMeta `json:",inline"`

	Name              string              `json:"name"`
	ExpirationSeconds int64               `json:"expiration_seconds"` // 默认 1 年
	Description       string              `json:"description"`
	PType             int                 `json:"p_type"` // 0 只读，1 自定义，2 管理员
	Rules             []rbacv1.PolicyRule `json:"rules"`  // p_type=1 时使用
	TargetNamespaces  []string            `json:"target_namespaces"`
	Force             bool                `json:"force"` // 强制下发
}

func (o *CreatePermissionRequest) SetDefaultOptions() {
	if o.ExpirationSeconds <= 0 {
		o.ExpirationSeconds = defaultExpirationSeconds
	}

	o.SANamespace = defaultNamespace
	o.SAName = fmt.Sprintf("pixiu-sa-%d", o.UserId)
	o.RoleBindingName = fmt.Sprintf("pixiu-rb-%d", o.UserId)
	switch o.PType {
	case 0:
		o.ClusterRoleName = PixiuViewClusterRole
	case 2:
		o.ClusterRoleName = ClusterAdminRole
	default:
		o.ClusterRoleName = fmt.Sprintf("pixiu-cr-%d", o.UserId)
	}
}

// Permission 集群 scoped kubeconfig 授权
type Permission struct {
	PixiuMeta `json:",inline"`
	TimeMeta  `json:",inline"`

	UserId            int64               `json:"user_id"`
	UserName          string              `json:"user_name"`
	ClusterId         int64               `json:"cluster_id"`
	ClusterName       string              `json:"cluster_name"`
	ClusterAliasName  string              `json:"cluster_alias_name"`
	Name              string              `json:"name"`
	ExpirationSeconds int64               `json:"expiration_seconds"`
	PType             int                 `json:"p_type"`
	Rules             []rbacv1.PolicyRule `json:"rules,omitempty"`
	SAName            string              `json:"sa_name"`
	SANamespace       string              `json:"sa_namespace"`
	TargetNamespaces  []string            `json:"target_namespaces"`
	KubeConfig        string              `json:"kube_config,omitempty"`
	Content           string              `json:"content,omitempty"` // 与 kube_config 相同，便于前端展示
	Description       string              `json:"description,omitempty"`
}

// KubeConfigResponse 返回给前端的 kubeconfig 内容
type KubeConfigResponse struct {
	ClusterName string `json:"cluster_name"`
	Content     string `json:"content"`
}

// ProxyKubeconfigResponse 指向 Pixiu 网关的标准 kubeconfig（token 仅返回一次）
type ProxyKubeconfigResponse struct {
	ClusterId          int64  `json:"cluster_id"`
	ClusterName        string `json:"cluster_name"`
	AliasName          string `json:"alias_name"`
	JTI                string `json:"jti"`
	ExpireAt           string `json:"expire_at"`
	Server             string `json:"server"`
	Token              string `json:"token"`
	KubeConfig         string `json:"kubeconfig"`
	KubeConfigEncoding string `json:"kubeconfig_encoding"` // yaml
}

// ProxyKubeconfigInfo 代理 kubeconfig 信息（不含 token 原文）。
type ProxyKubeconfigInfo struct {
	JTI       string `json:"jti"`
	Name      string `json:"name"`
	Server    string `json:"server"`
	ExpireAt  string `json:"expire_at"`
	CreatedAt string `json:"created_at"`
	IsActive  bool   `json:"is_active"`
}

type AgentHeartbeatRequest struct {
	Hostname string `json:"hostname"`
	Version  string `json:"version"`
}

type Job struct {
	Id       int64           `json:"id"`
	PlanId   int64           `json:"plan_id"`
	AgentId  int64           `json:"agent_id"`
	TaskName string          `json:"task_name"`
	Kind     model.JobKind   `json:"kind"`
	Action   string          `json:"action"`
	Image    string          `json:"image"`
	Payload  string          `json:"payload"`
	Status   model.JobStatus `json:"status"`
	Message  string          `json:"message"`
}

type AgentJobLogsRequest struct {
	Chunk string `json:"chunk"`
}

type AgentJobResultRequest struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Result  string `json:"result"` // 如 kubeconfig base64
}
