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
	"net"
	"testing"

	"github.com/minio/minio-go/v7"
)

func TestDefaultStorageRegion(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"oss-cn-hangzhou.aliyuncs.com", "cn-hangzhou"},
		{"oss-cn-hangzhou-internal.aliyuncs.com", "cn-hangzhou"},
		{"s3.oss-cn-hangzhou.aliyuncs.com", "cn-hangzhou"},
		{"s3.oss-cn-hangzhou-internal.aliyuncs.com", "cn-hangzhou"},
		{"cos.ap-guangzhou.myqcloud.com", "ap-guangzhou"},
		{"cos.ap-guangzhou.tencentcos.cn", "ap-guangzhou"},
		{"obs.cn-north-4.myhuaweicloud.com", "cn-north-4"},
		{"obs.eu-west-101.myhuaweicloud.eu", "eu-west-101"},
		{"s3.us-west-2.amazonaws.com", "us-west-2"},
		{"s3-us-west-2.amazonaws.com", "us-west-2"},
		{"s3.dualstack.ap-southeast-1.amazonaws.com", "ap-southeast-1"},
		{"s3-fips.us-east-1.amazonaws.com", "us-east-1"},
		{"s3.cn-north-1.amazonaws.com.cn", "cn-north-1"},
		// 公有云域名但不含地域（加速 / 全局 / 双栈端点）返回空串，交给 minio-go 探测
		{"oss-accelerate.aliyuncs.com", unmappedPublicCloudRegion},
		{"s3.oss-accelerate.aliyuncs.com", unmappedPublicCloudRegion},
		{"s3.amazonaws.com", unmappedPublicCloudRegion},
		{"cn-hangzhou.oss.aliyuncs.com", unmappedPublicCloudRegion},
		// 自建 / 未知域名维持原有兜底
		{"127.0.0.1", storageDefaultRegion},
		{"minio.example.com", storageDefaultRegion},
		{"oss-cn-hangzhou.notaliyuncs.com", storageDefaultRegion},
	}
	for _, tt := range tests {
		if got := defaultStorageRegion(tt.host); got != tt.want {
			t.Fatalf("defaultStorageRegion(%q)=%q, want %q", tt.host, got, tt.want)
		}
	}
}

func TestResolveBucketLookup(t *testing.T) {
	tests := []struct {
		style string
		host  string
		want  minio.BucketLookupType
	}{
		// 公有云未显式配置寻址方式时必须走虚拟托管：COS、OBS 已禁止 path 寻址。
		{"", "cos.ap-guangzhou.myqcloud.com", minio.BucketLookupDNS},
		{"", "cos.ap-guangzhou.tencentcos.cn", minio.BucketLookupDNS},
		{"", "obs.cn-north-4.myhuaweicloud.com", minio.BucketLookupDNS},
		{"", "oss-cn-hangzhou.aliyuncs.com", minio.BucketLookupDNS},
		{"", "s3.us-west-2.amazonaws.com", minio.BucketLookupDNS},
		// 自建存储仍交给 minio-go 自行探测；IP 端点物理上只能走路径方式。
		{"", "minio.example.com", minio.BucketLookupAuto},
		{"", "192.168.189.129", minio.BucketLookupAuto},
		{"virtual", "127.0.0.1", minio.BucketLookupPath},
		{"path", "cos.ap-guangzhou.myqcloud.com", minio.BucketLookupPath},
	}
	for _, tt := range tests {
		if got := resolveBucketLookup(tt.style, tt.host); got != tt.want {
			t.Fatalf("resolveBucketLookup(%q, %q)=%v, want %v", tt.style, tt.host, got, tt.want)
		}
	}
}

func TestNormalizeStorageEndpoint(t *testing.T) {
	tests := []struct {
		endpoint string
		want     string
	}{
		// 控制台给的 Bucket 域名要还原成服务级端点
		{"https://cos-test-1252082755.cos.ap-guangzhou.myqcloud.com", "https://cos.ap-guangzhou.myqcloud.com"},
		{"https://cos-test-1252082755.cos.ap-guangzhou.tencentcos.cn", "https://cos.ap-guangzhou.tencentcos.cn"},
		{"https://bucket.oss-cn-hangzhou.aliyuncs.com", "https://oss-cn-hangzhou.aliyuncs.com"},
		{"https://bucket.s3.oss-cn-hangzhou.aliyuncs.com", "https://s3.oss-cn-hangzhou.aliyuncs.com"},
		{"https://mybucket.s3.us-east-1.amazonaws.com", "https://s3.us-east-1.amazonaws.com"},
		{"https://bucket.obs.cn-north-4.myhuaweicloud.com", "https://obs.cn-north-4.myhuaweicloud.com"},
		{"http://bucket.obs.cn-north-4.myhuaweicloud.com:8080", "http://obs.cn-north-4.myhuaweicloud.com:8080"},
		// 服务级端点 / 自建端点原样保留
		{"https://cos.ap-guangzhou.myqcloud.com", "https://cos.ap-guangzhou.myqcloud.com"},
		{"https://s3.oss-cn-hangzhou.aliyuncs.com", "https://s3.oss-cn-hangzhou.aliyuncs.com"},
		{"https://minio.example.com:9000", "https://minio.example.com:9000"},
		{"https://files.mycorp.com", "https://files.mycorp.com"},
	}
	for _, tt := range tests {
		if got := normalizeStorageEndpoint(tt.endpoint); got != tt.want {
			t.Fatalf("normalizeStorageEndpoint(%q)=%q, want %q", tt.endpoint, got, tt.want)
		}
	}
}

func TestTencentCOSAppID(t *testing.T) {
	tests := []struct {
		endpoint string
		want     string
	}{
		{"https://cos-test-1252082755.cos.ap-guangzhou.myqcloud.com", "1252082755"},
		{"https://cos-test-1252082755.cos.ap-guangzhou.tencentcos.cn", "1252082755"},
		{"https://cos.ap-guangzhou.tencentcos.cn", ""},
	}
	for _, tt := range tests {
		if got := tencentCOSAppID("tencent", tt.endpoint); got != tt.want {
			t.Fatalf("tencentCOSAppID(%q)=%q, want %q", tt.endpoint, got, tt.want)
		}
	}
}

func TestNormalizeStorageEndpointForProvider(t *testing.T) {
	tests := []struct {
		provider string
		endpoint string
		want     string
	}{
		{"aliyun", "https://oss-cn-beijing.aliyuncs.com", "https://s3.oss-cn-beijing.aliyuncs.com"},
		{"aliyun", "https://bucket.oss-cn-beijing.aliyuncs.com", "https://s3.oss-cn-beijing.aliyuncs.com"},
		{"aliyun", "https://s3.oss-cn-beijing.aliyuncs.com", "https://s3.oss-cn-beijing.aliyuncs.com"},
		{"tencent", "https://oss-cn-beijing.aliyuncs.com", "https://oss-cn-beijing.aliyuncs.com"},
	}
	for _, tt := range tests {
		if got := normalizeStorageEndpointForProvider(tt.provider, tt.endpoint); got != tt.want {
			t.Fatalf("normalizeStorageEndpointForProvider(%q, %q)=%q, want %q", tt.provider, tt.endpoint, got, tt.want)
		}
	}
}

func TestNormalizeBucketNameForProvider(t *testing.T) {
	tests := []struct {
		provider string
		appID    string
		name     string
		want     string
		wantErr  bool
	}{
		{"tencent", "1252082755", "pixiu-static", "pixiu-static-1252082755", false},
		{"tencent", "1252082755", "pixiu-static-1252082755", "pixiu-static-1252082755", false},
		{"tencent", "", "pixiu-static", "", true},
		{"tencent", "", "pixiu-static-1252082755", "pixiu-static-1252082755", false},
		{"aliyun", "1252082755", "pixiu-static", "pixiu-static", false},
		{"huawei", "1252082755", "pixiu-static", "pixiu-static", false},
		{"aws", "1252082755", "pixiu-static", "pixiu-static", false},
	}
	for _, tt := range tests {
		got, err := normalizeBucketNameForProvider(tt.provider, tt.appID, tt.name)
		if (err != nil) != tt.wantErr {
			t.Fatalf("normalizeBucketNameForProvider(%q, %q, %q) error=%v, wantErr=%v", tt.provider, tt.appID, tt.name, err, tt.wantErr)
		}
		if got != tt.want {
			t.Fatalf("normalizeBucketNameForProvider(%q, %q, %q)=%q, want %q", tt.provider, tt.appID, tt.name, got, tt.want)
		}
	}
}

func TestS3ControlPlaneCapabilities(t *testing.T) {
	tests := []struct {
		provider     string
		supported    bool
		tagSupported bool
	}{
		{"minio", true, true},
		{"aws", true, true},
		{"s3", true, true},
		{"tencent", false, true},
		{"aliyun", false, false},
		{"huawei", false, true},
	}
	for _, tt := range tests {
		if got := supportsS3BucketPolicy(tt.provider); got != tt.supported {
			t.Fatalf("supportsS3BucketPolicy(%q)=%v, want %v", tt.provider, got, tt.supported)
		}
		if got := supportsS3BucketEncryption(tt.provider); got != tt.supported {
			t.Fatalf("supportsS3BucketEncryption(%q)=%v, want %v", tt.provider, got, tt.supported)
		}
		if got := supportsS3BucketTagging(tt.provider); got != tt.tagSupported {
			t.Fatalf("supportsS3BucketTagging(%q)=%v, want %v", tt.provider, got, tt.tagSupported)
		}
	}
}

func TestValidBucketName(t *testing.T) {
	ok := []string{"abc", "pixiu-static", "a.b-c1"}
	bad := []string{"", "ab", "ABucket", "192.168.1.1", "a..b", "-abc", "abc-"}
	for _, name := range ok {
		if !validBucketName(name) {
			t.Fatalf("validBucketName(%q)=false, want true", name)
		}
	}
	for _, name := range bad {
		if validBucketName(name) {
			t.Fatalf("validBucketName(%q)=true, want false", name)
		}
	}
}

func TestClassifyBucketPolicy(t *testing.T) {
	if got := classifyBucketPolicy(""); got != "private" {
		t.Fatalf("empty policy: got %q", got)
	}
	if got := classifyBucketPolicy("{not-json"); got != "private" {
		t.Fatalf("invalid json: got %q", got)
	}
	read := buildBucketPolicy("demo", "public-read")
	if got := classifyBucketPolicy(read); got != "public-read" {
		t.Fatalf("public-read: got %q policy=%s", got, read)
	}
	rw := buildBucketPolicy("demo", "public-read-write")
	if got := classifyBucketPolicy(rw); got != "public-read-write" {
		t.Fatalf("public-read-write: got %q", got)
	}
}

func TestSanitizeArchiveName(t *testing.T) {
	tests := map[string]string{
		"a/b.txt":            "a/b.txt",
		"../etc/passwd":      "etc/passwd",
		"/abs/path":          "abs/path",
		`..\windows\win.ini`: "windows/win.ini",
		".":                  "object",
		"":                   "object",
	}
	for in, want := range tests {
		if got := sanitizeArchiveName(in); got != want {
			t.Fatalf("sanitizeArchiveName(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestMatchVersionPrefix(t *testing.T) {
	if !matchVersionPrefix("a/b.txt", "a/b.txt") {
		t.Fatal("exact key should match")
	}
	if matchVersionPrefix("a/b.txt2", "a/b.txt") {
		t.Fatal("string prefix must not match sibling key")
	}
	if !matchVersionPrefix("a/b/c", "a/b") {
		t.Fatal("directory prefix should match children")
	}
	if !matchVersionPrefix("anything", "") {
		t.Fatal("empty prefix matches all")
	}
}

func TestBucketVirtualHostCapable(t *testing.T) {
	if bucketVirtualHostCapable("127.0.0.1") || bucketVirtualHostCapable("localhost") || bucketVirtualHostCapable("minio.localhost") {
		t.Fatal("ip/localhost must not use virtual-host")
	}
	if !bucketVirtualHostCapable("oss-cn-hangzhou.aliyuncs.com") {
		t.Fatal("real domain should allow virtual-host")
	}
}

func TestDisallowedProbeIP(t *testing.T) {
	if !disallowedProbeIP(net.ParseIP("127.0.0.1")) {
		t.Fatal("loopback should be disallowed")
	}
	if !disallowedProbeIP(net.ParseIP("169.254.169.254")) {
		t.Fatal("link-local metadata should be disallowed")
	}
	if disallowedProbeIP(net.ParseIP("10.0.0.1")) {
		t.Fatal("private nets must remain allowed for self-hosted MinIO")
	}
}

func TestCopyObjectTooLarge(t *testing.T) {
	if copyObjectTooLarge(maxCopyObjectBytes) {
		t.Fatal("exactly 5GiB should still use CopyObject")
	}
	if !copyObjectTooLarge(maxCopyObjectBytes + 1) {
		t.Fatal("over 5GiB must be rejected")
	}
}
