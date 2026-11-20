package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"SmsCodeWebhook/internal/auth"
	"SmsCodeWebhook/internal/parse"
	"SmsCodeWebhook/internal/store"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	store  *store.Store
	secret string
}

func New(s *store.Store, secret string) *Handler {
	return &Handler{store: s, secret: secret}
}

func param(c *gin.Context, name string) string {
	if v := strings.TrimSpace(c.PostForm(name)); v != "" {
		return v
	}
	return strings.TrimSpace(c.Query(name))
}

// KeyAuth 提取接口鉴权：X-API-Key 或 ?key=
func (h *Handler) KeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			key = c.Query("key")
		}
		if key == "" || key != h.secret {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "未授权"})
			return
		}
		c.Next()
	}
}

// Submit 提交短信：支持两种格式
// 1) JSON body：App 直接把 {"type","msg","phone"} 当 body 发过来（application/json）
// 2) 表单：from/content/timestamp/sign（验签）
func (h *Handler) Submit(c *gin.Context) {
	rawBody, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))

	var from, msgType, msg, phone string
	var ts int64

	if strings.Contains(c.Request.Header.Get("Content-Type"), "application/json") {
		var p parse.Content
		if err := json.Unmarshal(rawBody, &p); err != nil {
			log.Printf("解析 JSON body 失败: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "JSON body 不是合法 JSON: " + err.Error()})
			return
		}
		if p.Msg == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "msg 为空"})
			return
		}
		msgType, msg, phone = p.Type, p.Msg, p.Phone
		ts = time.Now().UnixMilli()
	} else {
		_ = c.Request.ParseForm()
		from = param(c, "from")
		contentRaw := param(c, "content")
		tsRaw := param(c, "timestamp")
		sign := param(c, "sign")

		if tsRaw == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "timestamp 必填"})
			return
		}
		if sign == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "sign 必填"})
			return
		}
		if !auth.VerifySign(sign, tsRaw, h.secret) {
			log.Printf("签名校验失败 from=%q timestamp=%q", from, tsRaw)
			c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "签名校验失败"})
			return
		}

		var err error
		msgType, msg, phone, err = parse.ContentField(contentRaw)
		if err != nil {
			log.Printf("解析 content 失败: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		ts, _ = strconv.ParseInt(tsRaw, 10, 64)
	}

	code, ok := parse.Code(msg)
	if !ok {
		log.Printf("未能提取验证码 type=%q phone=%q", msgType, phone)
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "未能从 msg 中提取验证码"})
		return
	}

	e := h.store.Put(from, msgType, phone, msg, code, ts)
	log.Printf("提交 type=%q phone=%q code=%q expire=%s", e.Type, e.Phone, e.Code, e.ExpireAt.Format(time.RFC3339))
	c.JSON(http.StatusOK, gin.H{
		"ok": true, "from": e.From, "type": e.Type, "phone": e.Phone, "code": e.Code,
		"content": e.Content, "timestamp": e.Timestamp,
		"created_at": e.CreatedAt, "expire_at": e.ExpireAt,
	})
}

// Extract 提取验证码：?type=&phone=&consume=true
func (h *Handler) Extract(c *gin.Context) {
	msgType := c.Query("type")
	phone := c.Query("phone")
	consume := c.DefaultQuery("consume", "true") != "false"

	e, ok := h.store.Get(msgType, phone, consume)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": "验证码不存在或已过期"})
		return
	}
	log.Printf("提取 type=%q phone=%q code=%q consume=%v", e.Type, e.Phone, e.Code, consume)
	c.JSON(http.StatusOK, gin.H{
		"ok": true, "from": e.From, "type": e.Type, "phone": e.Phone, "code": e.Code,
		"content": e.Content, "timestamp": e.Timestamp,
		"created_at": e.CreatedAt, "expire_at": e.ExpireAt, "consumed": consume,
	})
}
