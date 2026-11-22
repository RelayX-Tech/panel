// Package corecontrol 封装 Panel → core-control 的 gRPC 客户端。
package corecontrol

import (
	"context"

	corev1 "github.com/relayx-tech/contracts/gen/go/core/v1"
	"google.golang.org/grpc"
)

// Client 聚合 core-control 的 Config / Tunnel / Tools / Version / Health 客户端。
type Client struct {
	config  corev1.ConfigServiceClient
	tunnel  corev1.TunnelServiceClient
	tools   corev1.ToolsServiceClient
	version corev1.VersionServiceClient
	health  corev1.HealthServiceClient
}

// New 从已有连接构造客户端。
func New(conn grpc.ClientConnInterface) *Client {
	return &Client{
		config:  corev1.NewConfigServiceClient(conn),
		tunnel:  corev1.NewTunnelServiceClient(conn),
		tools:   corev1.NewToolsServiceClient(conn),
		version: corev1.NewVersionServiceClient(conn),
		health:  corev1.NewHealthServiceClient(conn),
	}
}

// Dial 建立到 core-control 的连接（grpc.NewClient，惰性连接）。
func Dial(target string, opts ...grpc.DialOption) (*Client, error) {
	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, err
	}
	return New(conn), nil
}

// Validate 字段级校验。
func (c *Client) Validate(ctx context.Context, req *corev1.ValidateRequest) (*corev1.ValidateResponse, error) {
	return c.config.Validate(ctx, req)
}

// Generate 生成单节点配置。
func (c *Client) Generate(ctx context.Context, req *corev1.GenerateRequest) (*corev1.GenerateResponse, error) {
	return c.config.Generate(ctx, req)
}

// GenerateBatch 批量生成。
func (c *Client) GenerateBatch(ctx context.Context, req *corev1.GenerateBatchRequest) (*corev1.GenerateBatchResponse, error) {
	return c.config.GenerateBatch(ctx, req)
}

// Preview 预览 / diff。
func (c *Client) Preview(ctx context.Context, req *corev1.PreviewRequest) (*corev1.PreviewResponse, error) {
	return c.config.Preview(ctx, req)
}

// GenerateTunnel 生成隧道配置与材料。
func (c *Client) GenerateTunnel(ctx context.Context, req *corev1.GenerateTunnelRequest) (*corev1.GenerateTunnelResponse, error) {
	return c.tunnel.GenerateTunnel(ctx, req)
}

// ScanReality Reality 扫描。
func (c *Client) ScanReality(ctx context.Context, req *corev1.ScanRealityRequest) (*corev1.ScanRealityResponse, error) {
	return c.tools.ScanReality(ctx, req)
}

// GenerateX25519 生成 X25519 密钥对。
func (c *Client) GenerateX25519(ctx context.Context, req *corev1.GenerateX25519Request) (*corev1.GenerateX25519Response, error) {
	return c.tools.GenerateX25519(ctx, req)
}

// ValidateCertificate 证书校验。
func (c *Client) ValidateCertificate(ctx context.Context, req *corev1.ValidateCertificateRequest) (*corev1.ValidateCertificateResponse, error) {
	return c.tools.ValidateCertificate(ctx, req)
}

// VersionInfo 返回 core / schema / contract / xray 版本。
func (c *Client) VersionInfo(ctx context.Context) (*corev1.VersionInfoResponse, error) {
	return c.version.Info(ctx, &corev1.VersionInfoRequest{})
}

// Health 返回 core-control 健康状态。
func (c *Client) Health(ctx context.Context) (*corev1.HealthCheckResponse, error) {
	return c.health.Check(ctx, &corev1.HealthCheckRequest{})
}
