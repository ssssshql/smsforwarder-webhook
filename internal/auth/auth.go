package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	KeyFileName = "api_key"
	keyBytes    = 8 // 8 字节 → 16 位 hex 密钥
)

// Sign 计算签名：urlEncode(base64(HmacSHA256(timestamp+"\n"+密钥, 密钥)))
func Sign(timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "\n" + secret))
	return url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}

// VerifySign 校验签名（兼容已 urlencode / 未 encode 的 base64）
func VerifySign(sign, timestamp, secret string) bool {
	if sign == "" || timestamp == "" {
		return false
	}
	expected := Sign(timestamp, secret)
	if hmac.Equal([]byte(sign), []byte(expected)) {
		return true
	}
	got, err := url.QueryUnescape(sign)
	if err != nil {
		got = sign
	}
	exp, _ := url.QueryUnescape(expected)
	return hmac.Equal([]byte(got), []byte(exp))
}

// ResolveSecret 环境变量 > 文件 > 随机生成并持久化
// 返回：密钥, 是否本次新生成
func ResolveSecret(dataDir string) (string, bool) {
	if key := os.Getenv("API_KEY"); key != "" {
		return key, false
	}
	path := filepath.Join(dataDir, KeyFileName)
	if data, err := os.ReadFile(path); err == nil {
		key := strings.TrimSpace(string(data))
		if key != "" {
			return key, false
		}
	}
	b := make([]byte, keyBytes)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("生成密钥失败: %v", err)
	}
	key := hex.EncodeToString(b)
	if err := os.WriteFile(path, []byte(key+"\n"), 0o600); err != nil {
		log.Fatalf("保存密钥失败: %v", err)
	}
	return key, true
}
