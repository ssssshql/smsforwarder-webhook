package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"SmsCodeWebhook/internal/auth"
	"SmsCodeWebhook/internal/handler"
	"SmsCodeWebhook/internal/store"

	"github.com/gin-gonic/gin"
)

const (
	defaultPort    = "8080"
	defaultDataDir = "/data"
	defaultTTL     = 5 * time.Minute
)

func main() {
	dataDir := envOr("DATA_DIR", defaultDataDir)
	port := envOr("PORT", defaultPort)
	ttl := defaultTTL
	if v := os.Getenv("CODE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			ttl = d
		} else {
			log.Printf("警告: CODE_TTL=%q 无效，使用默认 %s", v, defaultTTL)
		}
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	secret, generated := auth.ResolveSecret(dataDir)
	if generated {
		log.Printf("========================================")
		log.Printf("  首次启动，已生成密钥（请妥善保存）:")
		log.Printf("  %s", secret)
		log.Printf("  已写入: %s", filepath.Join(dataDir, auth.KeyFileName))
		log.Printf("========================================")
	}

	s := store.New(ttl)
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for range t.C {
			s.Cleanup()
		}
	}()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	h := handler.New(s, secret)
	api := r.Group("/api/v1")
	{
		// 提交：form from/content/timestamp/sign，content={"type":"...","msg":"..."}
		api.POST("/code", h.Submit)
		// 提取：?key=密钥 或 X-API-Key
		api.GET("/code", h.KeyAuth(), h.Extract)
	}

	addr := ":" + port
	log.Printf("短信验证码 Webhook 已启动，监听 %s，有效期=%s", addr, ttl)
	if err := r.Run(addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
