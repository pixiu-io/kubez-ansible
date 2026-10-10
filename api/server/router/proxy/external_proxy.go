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

package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	datasourceauth "github.com/caoyingjunz/pixiu/pkg/datasource/auth"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	externalProxyTargetQueryKey         = "url"
	externalProxyAuthorizationHeaderKey = "X-Pixiu-Proxy-Authorization"
)

const (
	externalProxyMaxBodyBytes          int64 = 10 << 20
	externalProxyDialTimeout                 = 10 * time.Second
	externalProxyTLSHandshakeTimeout         = 10 * time.Second
	externalProxyResponseHeaderTimeout       = 30 * time.Second
	externalProxyIdleConnTimeout             = 90 * time.Second
	externalProxyRequestTimeout              = 60 * time.Second
)

var errExternalProxyResponseTooLarge = errors.New("external proxy response exceeds size limit")
var errExternalProxyRequestTooLarge = errors.New("external proxy request exceeds size limit")

var externalProxyTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout: externalProxyDialTimeout,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       externalProxyIdleConnTimeout,
	TLSHandshakeTimeout:   externalProxyTLSHandshakeTimeout,
	ExpectContinueTimeout: 1 * time.Second,
	ResponseHeaderTimeout: externalProxyResponseHeaderTimeout,
}

func (p *proxyRouter) externalProxyHandler(c *gin.Context) {
	resp := httputils.NewResponse()
	ctx, cancel := context.WithTimeout(c.Request.Context(), externalProxyRequestTimeout)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	if c.Request.ContentLength > externalProxyMaxBodyBytes {
		httputils.SetFailed(c, resp, errExternalProxyRequestTooLarge)
		return
	}
	if c.Request.Body != nil {
		c.Request.Body = newMaxBytesReadCloser(c.Request.Body, externalProxyMaxBodyBytes, errExternalProxyRequestTooLarge)
	}

	var req struct {
		Act string `uri:"act"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	// gin 路由参数会解码转义字符（如 %2F → /），导致 RabbitMQ 默认 vhost 路径失真，
	// 因此转发路径改用原始转义形式重建，解码形式仅用于校验。
	escapedAct := escapedProxyAct(c)
	target, err := resolveExternalProxyTarget(c, req.Act, escapedAct)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	datasource, err := p.resolveProxyDatasource(c)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	// 外部代理强制要求数据源上下文，缺失即拒绝（见 requireExternalDatasourceContext）
	if err := requireExternalDatasourceContext(datasource); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := datasourceauth.ValidateExternalTarget(datasource, target); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	datasourceAuth, err := p.prepareExternalDatasourceRequest(c, target, datasource)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	p.forwardExternalRequest(c, resp, target, c.Request, datasourceAuth)
}

// requireExternalDatasourceContext 校验外部代理必须携带数据源上下文（X-Pixiu-Datasource-Id）。
// 目标校验 ValidateExternalTarget 完全依赖数据源配置的 URL 绑定：无数据源时目标退化为任意
// http/https URL，任意已登录用户可借服务端探测内网（SSRF），因此缺失数据源时必须拒绝。
// 注意：该策略仅适用于 /pixiu/external；/pixiu/proxy 的 kubernetes 代理无数据源属合法功能。
func requireExternalDatasourceContext(datasource *types.Datasource) error {
	if datasource != nil {
		return nil
	}
	return apierrors.NewError(
		fmt.Errorf("external proxy requires a datasource context (%s header)", upstreamDatasourceIDHeader),
		http.StatusForbidden,
	)
}

func resolveExternalProxyTarget(c *gin.Context, act, escapedAct string) (*url.URL, error) {
	raw := strings.TrimSpace(c.Query(externalProxyTargetQueryKey))
	if raw == "" {
		return nil, fmt.Errorf("missing %s query parameter", externalProxyTargetQueryKey)
	}

	baseURL, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", externalProxyTargetQueryKey, err)
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, fmt.Errorf("%s must use http or https scheme", externalProxyTargetQueryKey)
	}
	if baseURL.Host == "" {
		return nil, fmt.Errorf("%s must include host", externalProxyTargetQueryKey)
	}

	targetURL := *baseURL
	targetURL.Path = joinUpstreamProxyPath(baseURL.Path, act)
	targetURL.RawPath = joinUpstreamProxyPath(baseURL.EscapedPath(), escapedAct)

	query := c.Request.URL.Query()
	query.Del(externalProxyTargetQueryKey)
	mergedQuery := baseURL.Query()
	for key, values := range query {
		mergedQuery.Del(key)
		for _, value := range values {
			mergedQuery.Add(key, value)
		}
	}
	targetURL.RawQuery = mergedQuery.Encode()
	return &targetURL, nil
}

func (p *proxyRouter) forwardExternalRequest(c *gin.Context, resp *httputils.Response, targetURL *url.URL, upstreamReq *http.Request, datasourceAuth string) {
	reverseProxy := httputil.NewSingleHostReverseProxy(targetURL)
	reverseProxy.Transport = externalProxyTransport
	reverseProxy.Director = func(r *http.Request) {
		r.URL.Scheme = targetURL.Scheme
		r.URL.Host = targetURL.Host
		r.URL.Path = targetURL.Path
		r.URL.RawPath = targetURL.RawPath
		r.URL.RawQuery = targetURL.RawQuery
		r.Host = targetURL.Host
		r.Method = upstreamReq.Method

		r.Header = make(http.Header)
		for key, values := range upstreamReq.Header {
			lowerKey := strings.ToLower(strings.TrimSpace(key))
			if lowerKey == "authorization" || lowerKey == "cookie" || lowerKey == strings.ToLower(externalProxyAuthorizationHeaderKey) || lowerKey == strings.ToLower(upstreamDatasourceIDHeader) {
				continue
			}
			for _, value := range values {
				r.Header.Add(key, value)
			}
		}

		if datasourceAuth != "" {
			r.Header.Set("Authorization", datasourceAuth)
		} else if proxyAuth := strings.TrimSpace(upstreamReq.Header.Get(externalProxyAuthorizationHeaderKey)); proxyAuth != "" {
			r.Header.Set("Authorization", proxyAuth)
		}
	}
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		httputils.SetFailed(c, resp, proxyErr)
	}
	reverseProxy.ModifyResponse = func(r *http.Response) error {
		if r != nil && r.Body != nil {
			if r.ContentLength > externalProxyMaxBodyBytes {
				r.Body.Close()
				return errExternalProxyResponseTooLarge
			}
			r.Body = newMaxBytesReadCloser(r.Body, externalProxyMaxBodyBytes, errExternalProxyResponseTooLarge)
		}
		return nil
	}
	reverseProxy.ServeHTTP(c.Writer, upstreamReq)
}

type maxBytesReadCloser struct {
	rc        io.ReadCloser
	remaining int64
	limitErr  error
}

func newMaxBytesReadCloser(rc io.ReadCloser, limit int64, limitErr error) io.ReadCloser {
	return &maxBytesReadCloser{
		rc:        rc,
		remaining: limit,
		limitErr:  limitErr,
	}
}

func (m *maxBytesReadCloser) Read(p []byte) (int, error) {
	if m.remaining <= 0 {
		return 0, m.limitErr
	}

	maxRead := len(p)
	if int64(maxRead) > m.remaining {
		maxRead = int(m.remaining)
	}

	n, err := m.rc.Read(p[:maxRead])
	m.remaining -= int64(n)
	if err != nil {
		return n, err
	}

	if m.remaining == 0 {
		var probe [1]byte
		probeN, probeErr := m.rc.Read(probe[:])
		if probeN > 0 || (probeErr != nil && !errors.Is(probeErr, io.EOF)) {
			return n, m.limitErr
		}
		return n, io.EOF
	}

	return n, nil
}

func (m *maxBytesReadCloser) Close() error {
	return m.rc.Close()
}

// escapedProxyAct 返回外部代理路由的原始转义上游路径（保留 %2F 等转义字符）。
// gin 的 *act 路由参数已被解码，不能直接用于转发。
func escapedProxyAct(c *gin.Context) string {
	escaped := strings.TrimPrefix(c.Request.URL.EscapedPath(), externalProxyBaseURL)
	if escaped == "" {
		escaped = "/"
	}
	return escaped
}

func joinUpstreamProxyPath(basePath string, act string) string {
	trimmedBase := strings.TrimRight(basePath, "/")
	trimmedAct := "/" + strings.TrimLeft(act, "/")
	if trimmedBase == "" {
		return trimmedAct
	}
	if trimmedAct == "/" {
		return trimmedBase
	}
	return trimmedBase + trimmedAct
}
