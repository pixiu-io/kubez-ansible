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

package apps

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"net/http"
	"testing"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
)

// TestMaxUploadBytes 验证限额单一出处：已配置取配置值，未配置/非正值统一回退默认 32MB。
func TestMaxUploadBytes(t *testing.T) {
	cases := []struct {
		name       string
		configured int
		want       int64
	}{
		{name: "已配置 64MB", configured: 64, want: 64 << 20},
		{name: "已配置 1MB", configured: 1, want: 1 << 20},
		{name: "未配置回退默认 32MB", configured: 0, want: defaultMaxUploadMB << 20},
		{name: "负值回退默认 32MB", configured: -1, want: defaultMaxUploadMB << 20},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &controller{cc: config.Config{Appstore: config.AppstoreOptions{MaxUploadMB: tc.configured}}}
			if got := c.maxUploadBytes(); got != tc.want {
				t.Fatalf("maxUploadBytes() = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestParseChart 验证上传与导入共用的解析校验：合法 application 通过，library 与非包内容返回 400。
func TestParseChart(t *testing.T) {
	applicationYAML := "apiVersion: v2\nname: demo\nversion: 0.1.0\nappVersion: \"1.0\"\ndescription: demo chart\ntype: application\n"
	libraryYAML := "apiVersion: v2\nname: demo\nversion: 0.1.0\ntype: library\n"

	t.Run("合法 application 包", func(t *testing.T) {
		md, err := parseChart(buildChartTgz(t, "demo", applicationYAML))
		if err != nil {
			t.Fatalf("parseChart() unexpected error: %v", err)
		}
		if md.Name != "demo" || md.Version != "0.1.0" || md.Type != "application" {
			t.Fatalf("unexpected metadata: %+v", md)
		}
		if md.Sha256 == "" || md.Size == 0 {
			t.Fatalf("sha256/size not computed: %+v", md)
		}
	})

	t.Run("library 类型拒绝", func(t *testing.T) {
		_, err := parseChart(buildChartTgz(t, "demo", libraryYAML))
		requireAPIError(t, err, http.StatusBadRequest)
	})

	t.Run("非 chart 内容拒绝", func(t *testing.T) {
		_, err := parseChart([]byte("not-a-chart-archive"))
		requireAPIError(t, err, http.StatusBadRequest)
	})
}

// buildChartTgz 构造最小合法 Helm chart 包（<name>/Chart.yaml + 一个模板文件）。
func buildChartTgz(t *testing.T, name, chartYAML string) []byte {
	t.Helper()

	entries := []struct {
		name string
		body string
	}{
		{name: name + "/Chart.yaml", body: chartYAML},
		{name: name + "/templates/configmap.yaml", body: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + name + "\n"},
	}

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)
	for _, e := range entries {
		if err := tarWriter.WriteHeader(&tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body))}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tarWriter.Write([]byte(e.body)); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func requireAPIError(t *testing.T, err error, code int) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error with code %d, got nil", code)
	}
	var apiErr apierrors.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected apierrors.Error, got %T: %v", err, err)
	}
	if apiErr.Code != code {
		t.Fatalf("expected code %d, got %d (%v)", code, apiErr.Code, err)
	}
}
