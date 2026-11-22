package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	agentv1 "github.com/relayx-tech/contracts/gen/go/agent/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AgentClient 是 AgentApplier 依赖的最小 Agent 客户端能力（agentclient.Client 满足）。
type AgentClient interface {
	Apply(ctx context.Context, req *agentv1.AgentApplyRequest) (*agentv1.AgentApplyResponse, error)
	Rollback(ctx context.Context, req *agentv1.AgentRollbackRequest) (*agentv1.AgentRollbackResponse, error)
}

// AgentProvider 按节点返回 Agent 客户端（每节点一个）。
type AgentProvider func(nodeID string) (AgentClient, error)

// AgentApplier 是 orchestrator.Applier 的生产实现：把 NodePlan 翻译为 Agent 合同请求。
type AgentApplier struct {
	provider  AgentProvider
	now       func() time.Time
	requestID func() string
}

// NewAgentApplier 构造生产 Applier。
func NewAgentApplier(provider AgentProvider) *AgentApplier {
	return &AgentApplier{provider: provider, now: time.Now, requestID: randomRequestID}
}

// SetClock 覆盖时钟（测试用）。
func (a *AgentApplier) SetClock(now func() time.Time) {
	if now != nil {
		a.now = now
	}
}

// SetRequestID 覆盖 request_id 生成（测试用）。
func (a *AgentApplier) SetRequestID(fn func() string) {
	if fn != nil {
		a.requestID = fn
	}
}

// Apply 下发单节点配置。
func (a *AgentApplier) Apply(ctx context.Context, plan NodePlan) (*agentv1.AgentApplyResponse, error) {
	client, err := a.provider(plan.NodeID)
	if err != nil {
		return nil, err
	}
	return client.Apply(ctx, &agentv1.AgentApplyRequest{
		Meta: &agentv1.AgentRequestMeta{
			RequestId:   a.requestID(),
			OperationId: plan.OperationID,
			Timestamp:   timestamppb.New(a.now().UTC()),
		},
		NodeId:        plan.NodeID,
		Version:       plan.Version,
		ConfigHash:    plan.ConfigHash,
		ConfigJson:    plan.ConfigJSON,
		Materials:     plan.Materials,
		SchemaVersion: plan.SchemaVersion,
		XrayVersion:   plan.XrayVersion,
	})
}

// Rollback 回滚单节点到目标内容。
func (a *AgentApplier) Rollback(ctx context.Context, plan NodePlan, reason string) (*agentv1.AgentRollbackResponse, error) {
	client, err := a.provider(plan.NodeID)
	if err != nil {
		return nil, err
	}
	return client.Rollback(ctx, &agentv1.AgentRollbackRequest{
		Meta: &agentv1.AgentRequestMeta{
			RequestId:   a.requestID(),
			OperationId: plan.OperationID,
			Timestamp:   timestamppb.New(a.now().UTC()),
		},
		NodeId:        plan.NodeID,
		TargetVersion: plan.RollbackFrom,
		NewVersion:    plan.Version,
		ConfigHash:    plan.ConfigHash,
		ConfigJson:    plan.ConfigJSON,
		Materials:     plan.Materials,
		Reason:        reason,
	})
}

func randomRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "orchestrator"
	}
	return hex.EncodeToString(b)
}

var _ Applier = (*AgentApplier)(nil)
