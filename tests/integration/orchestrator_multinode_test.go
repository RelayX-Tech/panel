package integration

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"

	agentv1 "github.com/relayx-tech/contracts/gen/go/agent/v1"
	agentclient "github.com/relayx-tech/panel/internal/client/agent"
	"github.com/relayx-tech/panel/internal/orchestrator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// fakeAgent 是 AgentControlService 的测试实现（真实 gRPC 服务端）。
type fakeAgent struct {
	agentv1.UnimplementedAgentControlServiceServer
	mu      sync.Mutex
	fail    bool
	applied []string
	rolled  []string
}

func (f *fakeAgent) Apply(_ context.Context, req *agentv1.AgentApplyRequest) (*agentv1.AgentApplyResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return nil, fmt.Errorf("apply failed on %s", req.GetNodeId())
	}
	f.applied = append(f.applied, req.GetNodeId())
	return &agentv1.AgentApplyResponse{Accepted: true, State: "APPLIED", Version: req.GetVersion()}, nil
}

func (f *fakeAgent) Rollback(_ context.Context, req *agentv1.AgentRollbackRequest) (*agentv1.AgentRollbackResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rolled = append(f.rolled, req.GetNodeId())
	return &agentv1.AgentRollbackResponse{Accepted: true, State: "APPLIED", Version: req.GetNewVersion()}, nil
}

func (f *fakeAgent) snapshot() (applied, rolled []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.applied...), append([]string(nil), f.rolled...)
}

// startAgentServer 在 127.0.0.1:0 启动真实 gRPC Agent，返回地址。
func startAgentServer(t *testing.T, f *fakeAgent) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	agentv1.RegisterAgentControlServiceServer(srv, f)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func buildOrchestrator(t *testing.T, nodes []struct {
	id   string
	fail bool
}) (*orchestrator.Orchestrator, map[string]*fakeAgent) {
	t.Helper()
	clients := map[string]orchestrator.AgentClient{}
	fakes := map[string]*fakeAgent{}
	for _, n := range nodes {
		f := &fakeAgent{fail: n.fail}
		addr := startAgentServer(t, f)
		client, err := agentclient.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatal(err)
		}
		clients[n.id] = client
		fakes[n.id] = f
	}
	applier := orchestrator.NewAgentApplier(func(nodeID string) (orchestrator.AgentClient, error) {
		return clients[nodeID], nil
	})
	return orchestrator.New(applier), fakes
}

func plansFor(nodes []struct {
	id   string
	fail bool
}) []orchestrator.NodePlan {
	plans := make([]orchestrator.NodePlan, 0, len(nodes))
	for i, n := range nodes {
		plans = append(plans, orchestrator.NodePlan{
			NodeID: n.id, Version: uint64(i + 1), ConfigHash: "h", ConfigJSON: "{}", OperationID: "op-" + n.id,
		})
	}
	return plans
}

// 多节点全部成功：按序应用，无回滚。
func TestMultiNodeRollingSuccess(t *testing.T) {
	nodes := []struct {
		id   string
		fail bool
	}{{"n1", false}, {"n2", false}, {"n3", false}}
	orch, fakes := buildOrchestrator(t, nodes)

	res := orch.Rollout(context.Background(), plansFor(nodes))
	if res.Err != nil || res.Failed != nil {
		t.Fatalf("rollout res = %+v", res)
	}
	if len(res.Applied) != 3 {
		t.Fatalf("applied = %d", len(res.Applied))
	}
	for _, n := range nodes {
		applied, rolled := fakes[n.id].snapshot()
		if len(applied) != 1 || len(rolled) != 0 {
			t.Fatalf("节点 %s applied=%v rolled=%v", n.id, applied, rolled)
		}
	}
}

// 中途失败：停止后续节点并对已应用节点回滚。
func TestMultiNodeRollingStopsAndRollsBack(t *testing.T) {
	nodes := []struct {
		id   string
		fail bool
	}{{"n1", false}, {"n2", true}, {"n3", false}}
	orch, fakes := buildOrchestrator(t, nodes)

	res := orch.Rollout(context.Background(), plansFor(nodes))
	if res.Err == nil || res.Failed == nil || res.Failed.NodeID != "n2" {
		t.Fatalf("res = %+v", res)
	}
	if len(res.Applied) != 1 || res.Applied[0].NodeID != "n1" {
		t.Fatalf("applied = %+v", res.Applied)
	}
	if n3Applied, _ := fakes["n3"].snapshot(); len(n3Applied) != 0 {
		t.Fatalf("失败后不应应用 n3: %v", n3Applied)
	}
	if _, n1Rolled := fakes["n1"].snapshot(); len(n1Rolled) != 1 {
		t.Fatalf("n1 应被回滚: %v", n1Rolled)
	}
}
