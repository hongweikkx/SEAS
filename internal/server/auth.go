package server

import (
	"crypto/sha1"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

// wechatToken 从环境变量读取微信消息验证 Token
func wechatToken() string {
	if t := os.Getenv("WECHAT_TOKEN"); t != "" {
		return t
	}
	return "seas_dev_token"
}

// ========================================
// 微信消息结构
// ========================================

type WechatMsg struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgId        int64    `xml:"MsgId"`
	Event        string   `xml:"Event"`       // subscribe, SCAN
	EventKey     string   `xml:"EventKey"`    // qrscene_xxx
}

type WechatReply struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
}

// ========================================
// 微信签名验证
// ========================================

func verifyWechatSignature(token, signature, timestamp, nonce string) bool {
	tmpArr := []string{token, timestamp, nonce}
	sort.Strings(tmpArr)
	tmpStr := strings.Join(tmpArr, "")
	h := sha1.New()
	h.Write([]byte(tmpStr))
	computed := fmt.Sprintf("%x", h.Sum(nil))
	return computed == signature
}

// ========================================
// AuthHandler: 微信回调 + SSE
// ========================================

type AuthHandler struct {
	uc     *biz.AuthUsecase
	logger *log.Helper
}

// NewAuthHandler 创建认证 HTTP Handler
func NewAuthHandler(uc *biz.AuthUsecase, logger log.Logger) *AuthHandler {
	return &AuthHandler{
		uc:     uc,
		logger: log.NewHelper(logger),
	}
}

// ServeHTTP 分发微信回调和 SSE 请求
func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// GET 请求：微信服务器验证（配置回调 URL 时）
		h.handleWechatVerify(w, r)
	case http.MethodPost:
		// POST 请求：微信消息推送
		h.handleWechatMessage(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleWechatVerify 处理微信服务器验证请求
func (h *AuthHandler) handleWechatVerify(w http.ResponseWriter, r *http.Request) {
	signature := r.URL.Query().Get("signature")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")
	echostr := r.URL.Query().Get("echostr")

	if verifyWechatSignature(wechatToken(), signature, timestamp, nonce) {
		w.Write([]byte(echostr))
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

// handleWechatMessage 处理微信消息推送
func (h *AuthHandler) handleWechatMessage(w http.ResponseWriter, r *http.Request) {
	var msg WechatMsg
	if err := xml.NewDecoder(r.Body).Decode(&msg); err != nil {
		h.logger.Errorf("decode wechat message failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 只处理文本消息，且内容必须是 5 位纯数字（验证码）
	if msg.MsgType != "text" || len(msg.Content) != 5 || !isAllDigits(msg.Content) {
		reply := buildReply(msg.FromUserName, msg.ToUserName, "请回复页面上显示的 5 位验证码完成登录。")
		writeXML(w, reply)
		return
	}

	code := msg.Content
	openid := msg.FromUserName

	// 验证验证码并完成登录
	status, err := h.uc.VerifyLoginCode(r.Context(), code, openid)
	if err != nil {
		h.logger.Errorf("verify login code failed: %v", err)
		reply := buildReply(msg.FromUserName, msg.ToUserName, "登录失败，请稍后重试。")
		writeXML(w, reply)
		return
	}

	var replyContent string
	if status.Status == "expired" {
		replyContent = "验证码已过期，请刷新页面获取新验证码。"
	} else if status.Status == "success" {
		replyContent = "登录成功！您可以在浏览器中继续使用 SEAS 系统。"
	} else {
		replyContent = "登录处理中，请稍候..."
	}

	reply := buildReply(msg.FromUserName, msg.ToUserName, replyContent)
	writeXML(w, reply)
}

// isAllDigits 判断字符串是否全为数字
func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// buildReply 构建微信回复消息
func buildReply(toUser, fromUser, content string) WechatReply {
	return WechatReply{
		ToUserName:   toUser,
		FromUserName: fromUser,
		CreateTime:   time.Now().Unix(),
		MsgType:      "text",
		Content:      content,
	}
}

// writeXML 写入 XML 响应
func writeXML(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	data, _ := xml.Marshal(v)
	w.Write(data)
}

// ========================================
// SSE Handler: 登录状态推送
// ========================================

type LoginSSEHandler struct {
	uc     *biz.AuthUsecase
	logger *log.Helper
}

// NewLoginSSEHandler 创建 SSE Handler
func NewLoginSSEHandler(uc *biz.AuthUsecase, logger log.Logger) *LoginSSEHandler {
	return &LoginSSEHandler{
		uc:     uc,
		logger: log.NewHelper(logger),
	}
}

// ServeHTTP SSE 长连接推送登录状态
func (h *LoginSSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" || len(code) != 5 || !isAllDigits(code) {
		http.Error(w, "invalid code", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// 先发送 waiting 状态
	fmt.Fprintf(w, "event: status\ndata: %s\n\n", `{"status":"waiting"}`)
	flusher.Flush()

	// 轮询检查登录状态，最多 5 分钟
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ticker.C:
			status, err := h.uc.GetLoginStatus(r.Context(), code)
			if err != nil {
				h.logger.Errorf("get login status failed: %v", err)
				continue
			}

			data, _ := json.Marshal(status)
			fmt.Fprintf(w, "event: status\ndata: %s\n\n", data)
			flusher.Flush()

			// 登录成功或过期，结束连接
			if status.Status == "success" || status.Status == "expired" {
				return
			}

		case <-timeout:
			fmt.Fprintf(w, "event: status\ndata: %s\n\n", `{"status":"expired"}`)
			flusher.Flush()
			return

		case <-r.Context().Done():
			return
		}
	}
}
