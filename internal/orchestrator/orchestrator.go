// Package orchestrator 编排多节点滚动下发：1 节点 / 批，失败停止并对已应用节点回滚。
package orchestrator

import (
	"context"
	"fmt"

	agentv1 "github.com/relayx-tech/contracts/gen/go/agent/v1"
)

// NodePlan 是单节点下发计划。
type NodePlan struct {
	NodeID        string
	Version       uint64
	ConfigHash    string
	ConfigJSON    string
	Materials     []*agentv1.AgentMaterial
	SchemaVersion string
	XrayVersion   string
	OperationID   string
	RollbackFrom  uint64
}

// Applier 执行单节点应用 / 回滚（真实实现调 Agent 客户端；测试注入 fake）。
type Applier interface {
	Apply(ctx context.Context, plan NodePlan) (*agentv1.AgentApplyResponse, error)
	Rollback(ctx context.Context, plan NodePlan, reason string) (*agentv1.AgentRollbackResponse, error)
}

// Result 是滚动结果。
type Result struct {
	Applied    []NodePlan
	Failed     *NodePlan
	RolledBack []NodePlan
	Err        error
}

// Orchestrator 执行滚动下发。
type Orchestrator struct {
	applier Applier
}

// New 构造 Orchestrator。
func New(applier Applier) *Orchestrator {
	return &Orchestrator{applier: applier}
}

// Rollout 逐节点应用；任一节点失败立即停止后续，并对已应用节点按倒序回滚。
func (o *Orchestrator) Rollout(ctx context.Context, plans []NodePlan) *Result {
	res := &Result{}
	for i := range plans {
		plan := plans[i]
		resp, err := o.applier.Apply(ctx, plan)
		if err != nil || resp == nil || !resp.GetAccepted() {
			res.Failed = &plan
			if err != nil {
				res.Err = err
			} else {
				res.Err = fmt.Errorf("节点 %s 应用未接受", plan.NodeID)
			}
			res.RolledBack = o.rollbackApplied(ctx, res.Applied)
			return res
		}
		res.Applied = append(res.Applied, plan)
	}
	return res
}

func (o *Orchestrator) rollbackApplied(ctx context.Context, applied []NodePlan) []NodePlan {
	rolled := make([]NodePlan, 0, len(applied))
	for i := len(applied) - 1; i >= 0; i-- {
		plan := applied[i]
		if _, err := o.applier.Rollback(ctx, plan, "滚动失败后回滚"); err == nil {
			rolled = append(rolled, plan)
		}
	}
	return rolled
}
