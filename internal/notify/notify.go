// Package notify 实现告警通知渠道投递（Telegram / Webhook）。
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client 是通知投递客户端；Timeout 默认 10s。
type Client struct {
	HTTP    *http.Client
	BaseURL string // Telegram API 基地址（测试可覆盖）
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 10 * time.Second}, BaseURL: "https://api.telegram.org"}
}

// Send 按渠道类型投递一条测试消息。
func (c *Client) Send(ctx context.Context, kind, target, message string) error {
	switch strings.ToLower(kind) {
	case "webhook":
		return c.sendWebhook(ctx, target, message)
	case "telegram":
		return c.sendTelegram(ctx, target, message)
	default:
		return fmt.Errorf("unsupported channel type %q", kind)
	}
}

func (c *Client) sendWebhook(ctx context.Context, target, message string) error {
	body, _ := json.Marshal(map[string]string{"source": "relayx-panel", "text": message})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook responded with %d", resp.StatusCode)
	}
	return nil
}

// sendTelegram target 形如 "<botToken>:<chatId>"；botToken 自身含冒号，按最后一个冒号切分。
func (c *Client) sendTelegram(ctx context.Context, target, message string) error {
	idx := strings.LastIndex(target, ":")
	if idx <= 0 || idx == len(target)-1 {
		return fmt.Errorf("telegram target must be <botToken>:<chatId>")
	}
	token, chatID := target[:idx], target[idx+1:]
	endpoint := fmt.Sprintf("%s/bot%s/sendMessage", c.BaseURL, token)
	form := url.Values{"chat_id": {chatID}, "text": {message}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram responded with %d", resp.StatusCode)
	}
	return nil
}

// Mask 返回秘密字段的展示掩码（固定 6 圆点，design.md §8）。
func Mask(string) string { return "••••••" }
