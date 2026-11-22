package orchestrator_test

import (
	"context"
	"errors"
	"testing"

	agentv1 "github.com/relayx-tech/contracts/gen/go/agent/v1"
	"github.com/relayx-tech/panel/internal/orchestrator"
)

type fakeApplier struct {
	failNode string
	applied  []string
	rolled   []string
}

func (f *fakeApplier) Apply(_ context.Context, plan orchestrator.NodePlan) (*agentv1.AgentApplyResponse, error) {
	if plan.NodeID == f.failNode {
		return nil, errors.New("apply failed")
	}
	f.applied = append(f.applied, plan.NodeID)
	return &agentv1.AgentApplyResponse{Accepted: true, State: "APPLIED", Version: plan.Version}, nil
}

func (f *fakeApplier) Rollback(_ context.Context, plan orchestrator.NodePlan, _ string) (*agentv1.AgentRollbackResponse, error) {
	f.rolled = append(f.rolled, plan.NodeID)
	return &agentv1.AgentRollbackResponse{Accepted: true, State: "APPLIED"}, nil
}

func plans(ids ...string) []orchestrator.NodePlan {
	out := make([]orchestrator.NodePlan, 0, len(ids))
	for i, id := range ids {
		out = append(out, orchestrator.NodePlan{NodeID: id, Version: uint64(i + 1)})
	}
	return out
}

func TestRolloutAllSuccess(t *testing.T) {
	f := &fakeApplier{}
	res := orchestrator.New(f).Rollout(context.Background(), plans("n1", "n2", "n3"))
	if res.Err != nil || res.Failed != nil {
		t.Fatalf("res = %+v", res)
	}
	if len(res.Applied) != 3 || len(f.rolled) != 0 {
		t.Fatalf("applied=%d rolled=%d", len(res.Applied), len(f.rolled))
	}
}

func TestRolloutFailureStopsAndRollsBack(t *testing.T) {
	f := &fakeApplier{failNode: "n2"}
	res := orchestrator.New(f).Rollout(context.Background(), plans("n1", "n2", "n3"))
	if res.Err == nil || res.Failed == nil || res.Failed.NodeID != "n2" {
		t.Fatalf("res = %+v", res)
	}
	if len(res.Applied) != 1 || res.Applied[0].NodeID != "n1" {
		t.Fatalf("applied = %+v", res.Applied)
	}
	// n3 不应被应用；n1 应被回滚。
	for _, id := range f.applied {
		if id == "n3" {
			t.Fatal("失败后不应继续应用后续节点")
		}
	}
	if len(res.RolledBack) != 1 || res.RolledBack[0].NodeID != "n1" {
		t.Fatalf("rolledBack = %+v", res.RolledBack)
	}
}
