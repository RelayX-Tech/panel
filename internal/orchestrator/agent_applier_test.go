package orchestrator_test

import (
	"context"
	"testing"
	"time"

	agentv1 "github.com/relayx-tech/contracts/gen/go/agent/v1"
	"github.com/relayx-tech/panel/internal/orchestrator"
)

type fakeAgent struct {
	lastApply    *agentv1.AgentApplyRequest
	lastRollback *agentv1.AgentRollbackRequest
}

func (f *fakeAgent) Apply(_ context.Context, req *agentv1.AgentApplyRequest) (*agentv1.AgentApplyResponse, error) {
	f.lastApply = req
	return &agentv1.AgentApplyResponse{Accepted: true, State: "APPLIED", Version: req.GetVersion()}, nil
}

func (f *fakeAgent) Rollback(_ context.Context, req *agentv1.AgentRollbackRequest) (*agentv1.AgentRollbackResponse, error) {
	f.lastRollback = req
	return &agentv1.AgentRollbackResponse{Accepted: true, State: "APPLIED", Version: req.GetNewVersion()}, nil
}

func TestAgentApplierMapsPlan(t *testing.T) {
	fake := &fakeAgent{}
	applier := orchestrator.NewAgentApplier(func(string) (orchestrator.AgentClient, error) { return fake, nil })
	fixed := time.Unix(1000, 0)
	applier.SetClock(func() time.Time { return fixed })
	applier.SetRequestID(func() string { return "rid-1" })

	plan := orchestrator.NodePlan{
		NodeID: "node-1", Version: 3, ConfigHash: "h", ConfigJSON: `{"a":1}`,
		SchemaVersion: "1.0.0", XrayVersion: "26.11.9", OperationID: "op-1", RollbackFrom: 2,
	}
	resp, err := applier.Apply(context.Background(), plan)
	if err != nil || !resp.GetAccepted() {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	got := fake.lastApply
	if got.GetNodeId() != "node-1" || got.GetVersion() != 3 || got.GetConfigHash() != "h" ||
		got.GetMeta().GetOperationId() != "op-1" || got.GetMeta().GetRequestId() != "rid-1" {
		t.Fatalf("Apply 映射错误: %+v", got)
	}

	rb, err := applier.Rollback(context.Background(), plan, "manual")
	if err != nil || !rb.GetAccepted() {
		t.Fatalf("rollback resp=%+v err=%v", rb, err)
	}
	if fake.lastRollback.GetTargetVersion() != 2 || fake.lastRollback.GetNewVersion() != 3 || fake.lastRollback.GetReason() != "manual" {
		t.Fatalf("Rollback 映射错误: %+v", fake.lastRollback)
	}
}
