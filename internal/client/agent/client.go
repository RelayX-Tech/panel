// Package agent 封装 Panel → Agent 的 gRPC 客户端。
package agent

import (
	"context"

	agentv1 "github.com/relayx-tech/contracts/gen/go/agent/v1"
	"google.golang.org/grpc"
)

// Client 封装 AgentControlService 客户端。
type Client struct {
	ctl agentv1.AgentControlServiceClient
}

// New 从已有连接构造客户端。
func New(conn grpc.ClientConnInterface) *Client {
	return &Client{ctl: agentv1.NewAgentControlServiceClient(conn)}
}

// Dial 建立到 Agent 的连接（grpc.NewClient，惰性连接）。
func Dial(target string, opts ...grpc.DialOption) (*Client, error) {
	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, err
	}
	return New(conn), nil
}

// Hello 合同 / 版本握手。
func (c *Client) Hello(ctx context.Context, req *agentv1.HelloRequest) (*agentv1.HelloResponse, error) {
	return c.ctl.Hello(ctx, req)
}

// Apply 下发并应用配置。
func (c *Client) Apply(ctx context.Context, req *agentv1.AgentApplyRequest) (*agentv1.AgentApplyResponse, error) {
	return c.ctl.Apply(ctx, req)
}

// Rollback 回滚。
func (c *Client) Rollback(ctx context.Context, req *agentv1.AgentRollbackRequest) (*agentv1.AgentRollbackResponse, error) {
	return c.ctl.Rollback(ctx, req)
}

// Status 节点 / 内核 / 资源状态。
func (c *Client) Status(ctx context.Context, req *agentv1.StatusRequest) (*agentv1.StatusResponse, error) {
	return c.ctl.Status(ctx, req)
}

// Perception 内核感知。
func (c *Client) Perception(ctx context.Context, req *agentv1.AgentPerceptionRequest) (*agentv1.AgentPerceptionResponse, error) {
	return c.ctl.Perception(ctx, req)
}

// Interfaces 网卡清单。
func (c *Client) Interfaces(ctx context.Context, req *agentv1.InterfacesRequest) (*agentv1.InterfacesResponse, error) {
	return c.ctl.Interfaces(ctx, req)
}

// Probe TCP / HTTP 本地探测。
func (c *Client) Probe(ctx context.Context, req *agentv1.ProbeRequest) (*agentv1.ProbeResponse, error) {
	return c.ctl.Probe(ctx, req)
}

// Version Agent / core / Xray 版本。
func (c *Client) Version(ctx context.Context, req *agentv1.VersionRequest) (*agentv1.VersionResponse, error) {
	return c.ctl.Version(ctx, req)
}

// Health 存活 / 就绪。
func (c *Client) Health(ctx context.Context, req *agentv1.AgentHealthRequest) (*agentv1.AgentHealthResponse, error) {
	return c.ctl.Health(ctx, req)
}

// Update 下发 Agent 在线升级（STO-004，contract 1.4.0）。
func (c *Client) Update(ctx context.Context, req *agentv1.AgentUpdateRequest) (*agentv1.AgentUpdateResponse, error) {
	return c.ctl.Update(ctx, req)
}
