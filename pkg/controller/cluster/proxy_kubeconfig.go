/*
Copyright 2024 The Pixiu Authors.

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

package cluster

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
	"github.com/caoyingjunz/pixiu/pkg/util/token"
)

type proxyKubeconfig struct {
	c *cluster
}

func (c *cluster) ProxyKubeconfig() ProxyKubeconfigInterface {
	return &proxyKubeconfig{c: c}
}

// sealKey 返回访问令牌哈希的密封密钥：由服务端 jwt_key 派生（见 token.SealKeyFromJWTKey）。
// 密钥材料在服务端配置中、不落库，仅持有 DB 写权限无法伪造可过网关校验的 token 行。
func (p *proxyKubeconfig) sealKey() []byte {
	return token.SealKeyFromJWTKey([]byte(p.c.cc.Default.JWTKey))
}

// Create 签发 Access Token 并生成指向 Pixiu 的标准 kubeconfig。
func (p *proxyKubeconfig) Create(ctx context.Context, req *types.CreateProxyKubeconfigRequest) error {
	if !p.c.cc.KubeGateway.IsEnabled() {
		return errors.NewError(fmt.Errorf("kube-gateway is disabled"), http.StatusServiceUnavailable)
	}

	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return errors.ErrUnauthorized
	}
	obj, err := p.c.AuthorizeClusterKubeAccess(ctx, user, req.ClusterId)
	if err != nil {
		return err
	}

	// 每个用户每个集群只允许一个代理 token
	existing, _ := p.c.factory.Cluster().AccessToken().List(ctx, db.WithClusterId(obj.Id), db.WithUser(user.Id))
	if len(existing) > 0 {
		return errors.NewError(fmt.Errorf("proxy kubeconfig already exists"), http.StatusConflict)
	}

	gw := p.c.cc.KubeGateway
	gw.SetDefaults()
	now := time.Now()
	var expireAt time.Time
	if req.ExpiresAt != nil {
		expireAt = req.ExpiresAt.UTC()
		if !expireAt.After(now) {
			return errors.NewError(fmt.Errorf("expires_at must be in the future"), http.StatusBadRequest)
		}
		maxExpireAt := now.Add(time.Duration(gw.MaxExpireHours) * time.Hour)
		if expireAt.After(maxExpireAt) {
			return errors.NewError(fmt.Errorf("expires_at exceeds max %d hours", gw.MaxExpireHours), http.StatusBadRequest)
		}
	} else {
		expireAt = now.Add(time.Duration(gw.DefaultExpireHours) * time.Hour)
	}

	_, jti, hash, err := token.GenerateKubeAccessToken(p.sealKey())
	if err != nil {
		return errors.ErrServerInternal
	}
	record := &model.ClusterAccessToken{
		JTI:         jti,
		UserId:      user.Id,
		ClusterId:   obj.Id,
		ClusterName: obj.Name,
		TokenHash:   hash,
		ExpiresAt:   &expireAt,
	}
	if _, err = p.c.factory.Cluster().AccessToken().Create(ctx, record); err != nil {
		klog.Errorf("failed to create cluster access token: %v", err)
		return errors.ErrServerInternal
	}

	return nil
}

// Revoke 关闭外部访问：直接删除指定 jti 的访问令牌（仅 token 所有者）。
func (p *proxyKubeconfig) Revoke(ctx context.Context, clusterId int64, jti string) error {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return errors.ErrUnauthorized
	}
	if _, err = p.c.AuthorizeClusterAccess(ctx, user, clusterId); err != nil {
		return err
	}
	jti = strings.TrimSpace(jti)
	if jti == "" {
		return errors.ErrInvalidRequest
	}
	if err = p.c.factory.Cluster().AccessToken().Delete(ctx, db.WithJTI(jti), db.WithUser(user.Id)); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return errors.NewError(fmt.Errorf("access token not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to delete access token(%s): %v", jti, err)
		return errors.ErrServerInternal
	}
	return nil
}

// Get 获取集群的代理 kubeconfig 信息并签发新鲜 kubeconfig（每用户每集群仅一个）。
func (p *proxyKubeconfig) Get(ctx context.Context, clusterId int64) (*types.ProxyKubeconfigResponse, error) {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return nil, errors.ErrUnauthorized
	}
	obj, err := p.c.AuthorizeClusterAccess(ctx, user, clusterId)
	if err != nil {
		return nil, err
	}
	tokens, err := p.c.factory.Cluster().AccessToken().List(ctx, db.WithClusterId(clusterId), db.WithUser(user.Id))
	if err != nil {
		return nil, errors.ErrServerInternal
	}
	if len(tokens) == 0 {
		return nil, nil
	}
	t := tokens[0]

	gw := p.c.cc.KubeGateway
	gw.SetDefaults()
	plaintext, _, hash, err := token.GenerateKubeAccessToken(p.sealKey())
	if err != nil {
		return nil, errors.ErrServerInternal
	}
	if err = p.c.factory.Cluster().AccessToken().InternalUpdate(ctx, t.Id, map[string]interface{}{
		"token_hash": hash,
	}); err != nil {
		return nil, errors.ErrServerInternal
	}

	server, err := p.c.buildGatewayServer(ctx, t.ClusterName)
	if err != nil {
		return nil, errors.NewError(err, http.StatusInternalServerError)
	}
	kubeconfigYAML, err := renderProxyKubeconfig(obj, user.Id, server, plaintext, gw.InsecureSkipTLSVerify)
	if err != nil {
		return nil, errors.ErrServerInternal
	}
	expireAt := ""
	if t.ExpiresAt != nil {
		expireAt = t.ExpiresAt.Format(time.RFC3339)
	}
	return &types.ProxyKubeconfigResponse{
		ClusterId:          obj.Id,
		ClusterName:        obj.Name,
		AliasName:          obj.AliasName,
		JTI:                t.JTI,
		ExpireAt:           expireAt,
		Server:             server,
		Token:              plaintext,
		KubeConfig:         kubeconfigYAML,
		KubeConfigEncoding: "yaml",
	}, nil
}

// Validate 校验网关 Bearer token，返回用户与 token 记录。
func (p *proxyKubeconfig) Validate(ctx context.Context, plaintext string) (*model.User, *model.ClusterAccessToken, error) {
	plaintext = strings.TrimSpace(plaintext)
	if !token.IsKubeAccessToken(plaintext) {
		return nil, nil, errors.ErrUnauthorized
	}
	hash := token.HashKubeAccessToken(plaintext, p.sealKey())
	rec, err := p.c.factory.Cluster().AccessToken().GetBy(ctx, db.WithTokenHash(hash))
	if err != nil {
		klog.Errorf("failed to get access token by hash: %v", err)
		return nil, nil, errors.ErrServerInternal
	}
	if rec == nil || !rec.IsActive() {
		return nil, nil, errors.ErrUnauthorized
	}
	user, err := p.c.factory.User().Get(ctx, rec.UserId)
	if err != nil || user == nil {
		return nil, nil, errors.ErrUnauthorized
	}
	if user.Status == model.UserStatusForbidden {
		return nil, nil, errors.ErrForbidden
	}
	go func() {
		_ = p.c.factory.Cluster().AccessToken().TouchLastUsed(context.Background(), rec.Id)
	}()
	return user, rec, nil
}

func (c *cluster) buildGatewayServer(ctx context.Context, clusterName string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(c.cc.Default.PublicURL), "/")
	if base == "" {
		// 不从 127.0.0.1/localhost 推断，避免签发出本机地址的 kubeconfig
		base = inferPublicURL(ctx)
	}
	if base == "" {
		return "", fmt.Errorf("default.public_url is not configured; set it to the externally reachable HTTPS address (e.g. https://console.example.com)")
	}

	httpsPort := 8443
	if c.cc.TLS.Listen > 0 {
		httpsPort = c.cc.TLS.Listen
	}
	base, err := ensureHTTPSGatewayBase(base, httpsPort)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/k8s/%s", base, clusterName), nil
}

// renderProxyKubeconfig 生成经 Pixiu 网关访问的 kubeconfig 文档（YAML）。
// 集群/上下文名称使用集群 Name（ASCII），避免中文别名在部分 YAML/kubectl 环境下解析失败。
func renderProxyKubeconfig(obj *model.Cluster, userId int64, server, accessToken string, insecureSkipTLS bool) (string, error) {
	ctxName := strings.TrimSpace(obj.Name)
	if ctxName == "" {
		ctxName = strings.TrimSpace(obj.AliasName)
	}
	if ctxName == "" {
		ctxName = "pixiu-cluster"
	}
	userName := fmt.Sprintf("pixiu-%d", userId)

	cfg := clientcmdapi.NewConfig()
	clusterCfg := &clientcmdapi.Cluster{Server: server}
	if insecureSkipTLS {
		clusterCfg.InsecureSkipTLSVerify = true
	}
	cfg.Clusters[ctxName] = clusterCfg
	cfg.AuthInfos[userName] = &clientcmdapi.AuthInfo{Token: accessToken}
	cfg.Contexts[ctxName] = &clientcmdapi.Context{
		Cluster:  ctxName,
		AuthInfo: userName,
	}
	cfg.CurrentContext = ctxName

	b, err := clientcmd.Write(*cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func inferPublicURL(ctx context.Context) string {
	gc, ok := ctx.(*gin.Context)
	if !ok || gc == nil || gc.Request == nil {
		return ""
	}
	proto := gc.GetHeader("X-Forwarded-Proto")
	if proto == "" {
		if gc.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := gc.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = gc.Request.Host
	}
	if host == "" {
		return ""
	}
	// 取 hostname（去掉端口）判断是否回环地址
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}
	if isLoopbackHost(hostname) {
		return ""
	}
	return proto + "://" + host
}

func isLoopbackHost(host string) bool {
	h := strings.TrimSpace(strings.ToLower(host))
	if h == "" || h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
