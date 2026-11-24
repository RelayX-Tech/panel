package upgrade_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/upgrade"
)

// ---- fakes ----

type fakeUpgradeModel struct {
	model.UpgradeModel
	mu      sync.Mutex
	tasks   map[string]*model.UpgradeTask
	lock    *model.UpgradeLock
	targets map[string]model.UpgradeTarget
}

func newFakeUpgradeModel() *fakeUpgradeModel {
	return &fakeUpgradeModel{tasks: map[string]*model.UpgradeTask{}, targets: map[string]model.UpgradeTarget{}}
}

func (f *fakeUpgradeModel) CreateTask(_ context.Context, t *model.UpgradeTask) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	copy := *t
	f.tasks[t.ID] = &copy
	return nil
}
func (f *fakeUpgradeModel) GetTask(_ context.Context, id string) (*model.UpgradeTask, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tasks[id]
	if !ok {
		return nil, model.ErrNotFound
	}
	copy := *t
	copy.Nodes = append([]model.UpgradeTaskNode(nil), t.Nodes...)
	return &copy, nil
}
func (f *fakeUpgradeModel) ListTasks(context.Context, model.UpgradeTaskFilter) ([]model.UpgradeTask, int64, error) {
	return nil, 0, nil
}
func (f *fakeUpgradeModel) UpdateTaskStatus(_ context.Context, id, status, code, message string, startedAt, finishedAt *time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.tasks[id]; ok {
		t.Status, t.ErrorCode, t.ErrorMessage = status, code, message
		if startedAt != nil {
			t.StartedAt = startedAt
		}
		if finishedAt != nil {
			t.FinishedAt = finishedAt
		}
	}
	return nil
}
func (f *fakeUpgradeModel) UpsertTaskNode(_ context.Context, n *model.UpgradeTaskNode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tasks[n.TaskID]
	if !ok {
		return nil
	}
	for i := range t.Nodes {
		if t.Nodes[i].NodeID == n.NodeID {
			t.Nodes[i] = *n
			return nil
		}
	}
	t.Nodes = append(t.Nodes, *n)
	return nil
}
func (f *fakeUpgradeModel) ListTaskNodes(_ context.Context, id string) ([]model.UpgradeTaskNode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.tasks[id]; ok {
		return append([]model.UpgradeTaskNode(nil), t.Nodes...), nil
	}
	return nil, nil
}
func (f *fakeUpgradeModel) ActiveTask(context.Context) (*model.UpgradeTask, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.tasks {
		if t.Status == model.UpgradeTaskRunning || t.Status == model.UpgradeTaskQueued || t.Status == model.UpgradeTaskRollingBack {
			copy := *t
			return &copy, nil
		}
	}
	return nil, nil
}
func (f *fakeUpgradeModel) AcquireLock(_ context.Context, holder, scope string, now time.Time, ttl time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lock != nil && f.lock.ExpiresAt.After(now) && f.lock.Holder != holder {
		return false, nil
	}
	f.lock = &model.UpgradeLock{Holder: holder, Scope: scope, AcquiredAt: now, HeartbeatAt: now, ExpiresAt: now.Add(ttl)}
	return true, nil
}
func (f *fakeUpgradeModel) GetLock(context.Context) (*model.UpgradeLock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lock == nil {
		return nil, nil
	}
	copy := *f.lock
	return &copy, nil
}
func (f *fakeUpgradeModel) RenewLock(context.Context, string, time.Time, time.Duration) (bool, error) {
	return true, nil
}
func (f *fakeUpgradeModel) ReleaseLock(_ context.Context, holder string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lock != nil && f.lock.Holder == holder {
		f.lock = nil
	}
	return nil
}
func (f *fakeUpgradeModel) UpsertTarget(_ context.Context, target model.UpgradeTarget) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.targets[target.NodeID] = target
	return nil
}
func (f *fakeUpgradeModel) ListTargets(context.Context) ([]model.UpgradeTarget, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]model.UpgradeTarget, 0, len(f.targets))
	for _, t := range f.targets {
		out = append(out, t)
	}
	return out, nil
}
func (f *fakeUpgradeModel) DeleteTarget(_ context.Context, nodeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.targets, nodeID)
	return nil
}
func (f *fakeUpgradeModel) DeleteTargetsByBatch(_ context.Context, _ string) error { return nil }

type fakeNodes struct {
	model.NodeModel
	nodes map[string]model.NodeSummary
}

func (f fakeNodes) Get(_ context.Context, id string) (*model.NodeDetail, error) {
	n, ok := f.nodes[id]
	if !ok {
		return nil, model.ErrNotFound
	}
	return &model.NodeDetail{NodeSummary: n}, nil
}
func (f fakeNodes) List(context.Context, model.NodeFilter) ([]model.NodeSummary, int64, error) {
	out := make([]model.NodeSummary, 0, len(f.nodes))
	for _, n := range f.nodes {
		out = append(out, n)
	}
	return out, int64(len(out)), nil
}

type fakeAlerts struct {
	model.AlertModel
	raised []model.Alert
}

func (f *fakeAlerts) Upsert(_ context.Context, a model.Alert) (*model.Alert, error) {
	f.raised = append(f.raised, a)
	return &a, nil
}

type fakeAgents struct {
	updates []upgrade.AgentUpdateRequest
	failOn  map[string]bool
	status  string
}

func (f *fakeAgents) Update(_ context.Context, req upgrade.AgentUpdateRequest) (upgrade.AgentUpdateResult, error) {
	f.updates = append(f.updates, req)
	if f.failOn[req.NodeID] && req.Reason == "upgrade" {
		return upgrade.AgentUpdateResult{Accepted: true, State: model.NodeUpgradeApplying}, nil
	}
	return upgrade.AgentUpdateResult{Accepted: true, State: model.NodeUpgradeApplying}, nil
}
func (f *fakeAgents) Status(_ context.Context, nodeID string) (upgrade.AgentUpdateStatus, error) {
	if f.failOn[nodeID] {
		return upgrade.AgentUpdateStatus{State: model.NodeUpgradeFailed, ErrorCode: "IMAGE_PULL_FAILED"}, nil
	}
	return upgrade.AgentUpdateStatus{State: model.NodeUpgradeSucceeded}, nil
}

func onlineNode(id string) model.NodeSummary {
	now := time.Now().UTC()
	return model.NodeSummary{ID: id, Name: id, Status: "online", AgentVersion: "1.3.0", LastSeenAt: &now}
}

func newService(t *testing.T) (*upgrade.Service, *fakeUpgradeModel, *fakeAgents, *fakeAlerts) {
	t.Helper()
	models := newFakeUpgradeModel()
	now := time.Now().UTC()
	nodes := fakeNodes{nodes: map[string]model.NodeSummary{
		"n1": onlineNode("n1"),
		"n2": onlineNode("n2"),
	}}
	agents := &fakeAgents{failOn: map[string]bool{}}
	alerts := &fakeAlerts{}
	svc := &upgrade.Service{
		Model: models, Nodes: nodes, Alerts: alerts, Agents: agents,
		Holder: "panel/test", PanelVersion: "1.3.0", Channel: "stable",
		PanelMinor: "4", AgentMinor: "4", SchemaEqual: true,
		PollInterval: time.Millisecond, PollAttempts: 2,
		Now: func() time.Time { return now },
	}
	return svc, models, agents, alerts
}

// ---- tests ----

func TestStartAgentsSuccess(t *testing.T) {
	svc, models, agents, alerts := newService(t)
	task, err := svc.StartAgents(context.Background(), "admin", "req-1", "op-1", "1.4.0", []string{"n1", "n2"}, "", "")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if task.Status != model.UpgradeTaskSucceeded {
		full, _ := models.GetTask(context.Background(), task.ID)
		t.Fatalf("task status = %s, want succeeded (%+v)", task.Status, full)
	}
	if len(agents.updates) != 2 {
		t.Fatalf("updates = %d, want 2", len(agents.updates))
	}
	if len(alerts.raised) != 0 {
		t.Fatalf("success should not raise alerts: %+v", alerts.raised)
	}
	if lock, _ := models.GetLock(context.Background()); lock != nil {
		t.Fatal("lock should be released")
	}
}

func TestStartAgentsRollback(t *testing.T) {
	svc, models, agents, alerts := newService(t)
	agents.failOn["n2"] = true
	task, err := svc.StartAgents(context.Background(), "admin", "req-1", "op-1", "1.4.0", []string{"n1", "n2"}, "", "")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	full, _ := models.GetTask(context.Background(), task.ID)
	if full.Status != model.UpgradeTaskRolledBack {
		t.Fatalf("status = %s, want rolled_back", full.Status)
	}
	rollbackSeen := false
	for _, u := range agents.updates {
		if u.NodeID == "n1" && u.Reason == "rollback" {
			rollbackSeen = true
		}
	}
	if !rollbackSeen {
		t.Fatalf("n1 should be rolled back: %+v", agents.updates)
	}
	if len(alerts.raised) == 0 || alerts.raised[len(alerts.raised)-1].Level != "critical" {
		t.Fatalf("expected critical alert, got %+v", alerts.raised)
	}
}

func TestStartAgentsOfflineCreatesTarget(t *testing.T) {
	svc, models, _, _ := newService(t)
	now := time.Now().UTC().Add(-time.Hour)
	svc.Nodes = fakeNodes{nodes: map[string]model.NodeSummary{
		"n1": onlineNode("n1"),
		"n2": {ID: "n2", Name: "n2", Status: "offline", LastSeenAt: &now, AgentVersion: "1.3.0"},
	}}
	task, err := svc.StartAgents(context.Background(), "admin", "req-1", "op-1", "1.4.0", []string{"n1", "n2"}, "", "")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	full, _ := models.GetTask(context.Background(), task.ID)
	if full.Status != model.UpgradeTaskSucceeded {
		t.Fatalf("status = %s", full.Status)
	}
	targets, _ := models.ListTargets(context.Background())
	if len(targets) != 1 || targets[0].NodeID != "n2" {
		t.Fatalf("targets = %+v", targets)
	}
}

func TestStartPanelComposeRejected(t *testing.T) {
	svc, _, _, _ := newService(t)
	svc.ComposeMode = true
	if _, err := svc.StartPanel(context.Background(), "admin", "req-1", "op-1", "1.4.0"); !errors.Is(err, upgrade.ErrComposeManual) {
		t.Fatalf("err = %v, want ErrComposeManual", err)
	}
}

func TestStartPanelDowngradeRejected(t *testing.T) {
	svc, _, _, _ := newService(t)
	if _, err := svc.StartPanel(context.Background(), "admin", "req-1", "op-1", "1.2.0"); !errors.Is(err, upgrade.ErrDowngrade) {
		t.Fatalf("err = %v, want ErrDowngrade", err)
	}
}

func TestLockBusy(t *testing.T) {
	svc, models, _, _ := newService(t)
	now := time.Now().UTC()
	_, _ = models.AcquireLock(context.Background(), "other", "panel", now, time.Hour)
	if _, err := svc.StartAgents(context.Background(), "admin", "req-1", "op-1", "1.4.0", []string{"n1"}, "", ""); !errors.Is(err, upgrade.ErrLockBusy) {
		t.Fatalf("err = %v, want ErrLockBusy", err)
	}
}
