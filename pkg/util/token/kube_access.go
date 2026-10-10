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

package token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

const KubeAccessTokenPrefix = "pxk_"

// GenerateKubeAccessToken 生成 opaque 访问令牌：pxk_<jti>.<secret>
// 返回 plaintext、jti、tokenHash(HMAC-SHA256 hex，密钥为 sealKey)。
func GenerateKubeAccessToken(sealKey []byte) (plaintext, jti, tokenHash string, err error) {
	jtiBytes := make([]byte, 16)
	if _, err = rand.Read(jtiBytes); err != nil {
		return "", "", "", err
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return "", "", "", err
	}

	jti = hex.EncodeToString(jtiBytes)
	secretEnc := base64.RawURLEncoding.EncodeToString(secret)
	plaintext = KubeAccessTokenPrefix + jti + "." + secretEnc
	tokenHash = HashKubeAccessToken(plaintext, sealKey)
	return plaintext, jti, tokenHash, nil
}

// SealKeyFromJWTKey 由服务端 jwt_key 派生访问令牌哈希的密封密钥（HMAC-SHA256 key）。
// 密钥材料取自服务端配置（不落库），抬高仅持有 DB 写权限的攻击者伪造 token 行的门槛；
// 与 jwt_key 同生命周期：轮换 jwt_key 会使已签发 token 全部失效，需重新签发代理 kubeconfig。
func SealKeyFromJWTKey(jwtKey []byte) []byte {
	sum := sha256.Sum256([]byte("pixiu/kube-access-token-seal/v1:" + string(jwtKey)))
	return sum[:]
}

// HashKubeAccessToken 计算访问令牌哈希（HMAC-SHA256 hex），用于落库与校验比对。
// 不使用无盐裸 sha256：仅凭 DB 内容即可离线伪造令牌行。
// 按设计不包含旧版裸 sha256 哈希的兼容回退（有回退即等于未加固），
// 升级后旧 token 一律失效，用户重新签发代理 kubeconfig 即可。
func HashKubeAccessToken(plaintext string, sealKey []byte) string {
	mac := hmac.New(sha256.New, sealKey)
	// \x00 分隔域名前缀与令牌明文，避免不同用途的 HMAC 消息域混淆
	mac.Write([]byte("pixiu/kube-access-token/v1\x00" + plaintext))
	return hex.EncodeToString(mac.Sum(nil))
}

func IsKubeAccessToken(plaintext string) bool {
	return strings.HasPrefix(plaintext, KubeAccessTokenPrefix)
}
