package upgrade

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// UpgraderStatus 是 upgrader 状态（design.md §7.3）。
type UpgraderStatus struct {
	State       string         `json:"state"`
	OperationID string         `json:"operationId"`
	FromVersion string         `json:"fromVersion"`
	ToVersion   string         `json:"toVersion"`
	Phase       string         `json:"phase"`
	Progress    int            `json:"progress"`
	StartedAt   string         `json:"startedAt"`
	FinishedAt  string         `json:"finishedAt"`
	Error       *UpgraderError `json:"error,omitempty"`
}

type UpgraderError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Asset 是升级包资产。
type Asset struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	Arch      string `json:"arch"`
	Signature string `json:"signature,omitempty"`
}

// PanelUpgradeRequest 是 POST /upgrade 的请求体。
type PanelUpgradeRequest struct {
	OperationID       string  `json:"operationId"`
	TargetVersion     string  `json:"targetVersion"`
	ManifestURL       string  `json:"manifestUrl"`
	ManifestSignature string  `json:"manifestSignature"`
	Assets            []Asset `json:"assets"`
	DeadlineSeconds   int     `json:"deadlineSeconds"`
}

// RollbackRequest 是 POST /rollback 的请求体。
type RollbackRequest struct {
	OperationID   string `json:"operationId"`
	TargetVersion string `json:"targetVersion"`
}

// UpgraderClient 是 Panel 侧依赖的 upgrader 客户端接口。
type UpgraderClient interface {
	Status(ctx context.Context) (*UpgraderStatus, error)
	Upgrade(ctx context.Context, req PanelUpgradeRequest) error
	Rollback(ctx context.Context, req RollbackRequest) error
	Healthz(ctx context.Context) error
}

// UpgraderHTTP 通过 UDS + 共享令牌访问 relayx-upgrader。
type UpgraderHTTP struct {
	Endpoint string
	Token    string
	HTTP     *http.Client
}

var _ UpgraderClient = (*UpgraderHTTP)(nil)

// NewUpgraderHTTP 从 socket 路径与令牌文件构造客户端。
func NewUpgraderHTTP(socketPath, tokenFile string, timeout time.Duration) (*UpgraderHTTP, error) {
	token, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("read upgrader token: %w", err)
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		},
	}
	return &UpgraderHTTP{
		Endpoint: "http://unix",
		Token:    strings.TrimSpace(string(token)),
		HTTP:     &http.Client{Transport: transport, Timeout: timeout},
	}, nil
}

func (c *UpgraderHTTP) Status(ctx context.Context) (*UpgraderStatus, error) {
	var out UpgraderStatus
	if err := c.do(ctx, http.MethodGet, "/status", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *UpgraderHTTP) Upgrade(ctx context.Context, req PanelUpgradeRequest) error {
	var out struct {
		Accepted bool   `json:"accepted"`
		State    string `json:"state"`
	}
	if err := c.do(ctx, http.MethodPost, "/upgrade", req, &out); err != nil {
		return err
	}
	if !out.Accepted {
		return fmt.Errorf("upgrader rejected upgrade: %s", out.State)
	}
	return nil
}

func (c *UpgraderHTTP) Rollback(ctx context.Context, req RollbackRequest) error {
	var out struct {
		Accepted bool   `json:"accepted"`
		State    string `json:"state"`
	}
	return c.do(ctx, http.MethodPost, "/rollback", req, &out)
}

func (c *UpgraderHTTP) Healthz(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/healthz", nil, nil)
}

func (c *UpgraderHTTP) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Endpoint+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Relayx-Upgrader-Token", c.Token)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("upgrader request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("upgrader responded %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
