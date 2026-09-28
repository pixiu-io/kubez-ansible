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
)

func TestDefaultStorageRegion(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"oss-cn-hangzhou.aliyuncs.com", "cn-hangzhou"},
		{"oss-cn-hangzhou-internal.aliyuncs.com", "cn-hangzhou"},
		{"cos.ap-guangzhou.myqcloud.com", "ap-guangzhou"},
		{"obs.cn-north-4.myhuaweicloud.com", "cn-north-4"},
		{"s3.us-west-2.amazonaws.com", "us-west-2"},
		{"s3-us-west-2.amazonaws.com", "us-west-2"},
		{"s3.dualstack.ap-southeast-1.amazonaws.com", "ap-southeast-1"},
		{"s3.cn-north-1.amazonaws.com.cn", "cn-north-1"},
		{"s3.amazonaws.com", storageDefaultRegion},
		{"127.0.0.1", storageDefaultRegion},
		{"minio.example.com", storageDefaultRegion},
	}
	for _, tt := range tests {
		if got := defaultStorageRegion(tt.host); got != tt.want {
			t.Fatalf("defaultStorageRegion(%q)=%q, want %q", tt.host, got, tt.want)
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
