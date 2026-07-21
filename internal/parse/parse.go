package parse

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Content content 字段 JSON
// 示例: {"type":"疯狂水世界","msg":"【...】验证码为809758...","phone":"SIM1_152977314482"}
type Content struct {
	Type  string `json:"type"`
	Msg   string `json:"msg"`
	Phone string `json:"phone"`
}

// 验证码提取正则（按优先级）
var codeRe = []*regexp.Regexp{
	regexp.MustCompile(`验证码[为是：:\s]*([0-9]{4,8})`),
	regexp.MustCompile(`校验码[为是：:\s]*([0-9]{4,8})`),
	regexp.MustCompile(`动态码[为是：:\s]*([0-9]{4,8})`),
	regexp.MustCompile(`码[为是：:\s]*([0-9]{4,8})`),
	regexp.MustCompile(`(?i)code[：:\s]*([0-9]{4,8})`),
	regexp.MustCompile(`([0-9]{4,8})`),
}

// ContentField 解析 content 字段
func ContentField(raw string) (msgType, msg, phone string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", "", fmt.Errorf("content 为空")
	}
	var p Content
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return "", "", "", fmt.Errorf("content 不是合法 JSON: %w", err)
	}
	if p.Msg == "" {
		return "", "", "", fmt.Errorf("content.msg 为空")
	}
	return p.Type, p.Msg, p.Phone, nil
}

// Code 从短信正文提取验证码
func Code(text string) (string, bool) {
	for _, re := range codeRe {
		if m := re.FindStringSubmatch(text); len(m) >= 2 && m[1] != "" {
			return m[1], true
		}
	}
	return "", false
}
