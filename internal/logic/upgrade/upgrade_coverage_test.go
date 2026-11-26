package upgrade_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/logic/upgrade"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
	upgradepkg "github.com/relayx-tech/panel/internal/upgrade"
)

// ---- fakes ----

type upgradeModelFunc struct {
	model.UpgradeModel
	getLockFn            func(context.Context) (*model.UpgradeLock, error)
	activeTaskFn         func(context.Context) (*model.UpgradeTask, error)
	listTasksFn          func(context.Context, model.UpgradeTaskFilter) ([]model.UpgradeTask, int64, error)
	getTaskFn            func(context.Context, string) (*model.UpgradeTask, error)
	listTargetsFn        func(context.Context) ([]model.UpgradeTarget, error)
	acquireLockFn        func(context.Context, string, string, time.Time, time.Duration) (bool, error)
	createTaskFn         func(context.Context, *model.UpgradeTask) error
	updateTaskStatusFn   func(context.Context, string, string, string, string, *time.Time, *time.Time) error
	upsertTaskNodeFn     func(context.Context, *model.UpgradeTaskNode) error
	deleteTargetsByBatch func(context.Context, string) error
}

func (f upgradeModelFunc) GetLock(ctx context.Context) (*model.UpgradeLock, error) {
	if f.getLockFn != nil {
		return f.getLockFn(ctx)
	}
	return nil, nil
}
func (f upgradeModelFunc) ActiveTask(ctx context.Context) (*model.UpgradeTask, error) {
	if f.activeTaskFn != nil {
		return f.activeTaskFn(ctx)
	}
	return nil, nil
}
func (f upgradeModelFunc) ListTasks(ctx context.Context, filter model.UpgradeTaskFilter) ([]model.UpgradeTask, int64, error) {
	if f.listTasksFn != nil {
		return f.listTasksFn(ctx, filter)
	}
	return nil, 0, nil
}
func (f upgradeModelFunc) GetTask(ctx context.Context, id string) (*model.UpgradeTask, error) {
	if f.getTaskFn != nil {
		return f.getTaskFn(ctx, id)
	}
	return nil, model.ErrNotFound
}
func (f upgradeModelFunc) ListTargets(ctx context.Context) ([]model.UpgradeTarget, error) {
	if f.listTargetsFn != nil {
		return f.listTargetsFn(ctx)
	}
	return nil, nil
}
func (f upgradeModelFunc) AcquireLock(ctx context.Context, holder, scope string, now time.Time, ttl time.Duration) (bool, error) {
	if f.acquireLockFn != nil {
		return f.acquireLockFn(ctx, holder, scope, now, ttl)
	}
	return true, nil
}
func (f upgradeModelFunc) CreateTask(ctx context.Context, task *model.UpgradeTask) error {
	if f.createTaskFn != nil {
		return f.createTaskFn(ctx, task)
	}
	return nil
}
func (f upgradeModelFunc) UpdateTaskStatus(ctx context.Context, id, status, code, msg string, startedAt, finishedAt *time.Time) error {
	if f.updateTaskStatusFn != nil {
		return f.updateTaskStatusFn(ctx, id, status, code, msg, startedAt, finishedAt)
	}
	return nil
}
func (f upgradeModelFunc) UpsertTaskNode(ctx context.Context, n *model.UpgradeTaskNode) error {
	if f.upsertTaskNodeFn != nil {
		return f.upsertTaskNodeFn(ctx, n)
	}
	return nil
}
func (f upgradeModelFunc) ReleaseLock(context.Context, string) error { return nil }
func (f upgradeModelFunc) DeleteTargetsByBatch(ctx context.Context, batchID string) error {
	if f.deleteTargetsByBatch != nil {
		return f.deleteTargetsByBatch(ctx, batchID)
	}
	return nil
}

type nodeModelForUpgrade struct {
	model.NodeModel
	getFn func(context.Context, string) (*model.NodeDetail, error)
}

func (f nodeModelForUpgrade) Get(ctx context.Context, id string) (*model.NodeDetail, error) {
	if f.getFn != nil {
		return f.getFn(ctx, id)
	}
	return nil, model.ErrNotFound
}

type manifestSourceFunc struct {
	fetchFn func(context.Context, string) (*upgradepkg.ReleaseManifest, error)
}

func (f manifestSourceFunc) Fetch(ctx context.Context, channel string) (*upgradepkg.ReleaseManifest, error) {
	return f.fetchFn(ctx, channel)
}

type agentUpdaterFunc struct {
	updateFn func(context.Context, upgradepkg.AgentUpdateRequest) (upgradepkg.AgentUpdateResult, error)
	statusFn func(context.Context, string) (upgradepkg.AgentUpdateStatus, error)
}

func (f agentUpdaterFunc) Update(ctx context.Context, req upgradepkg.AgentUpdateRequest) (upgradepkg.AgentUpdateResult, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, req)
	}
	return upgradepkg.AgentUpdateResult{Accepted: true}, nil
}
func (f agentUpdaterFunc) Status(ctx context.Context, nodeID string) (upgradepkg.AgentUpdateStatus, error) {
	if f.statusFn != nil {
		return f.statusFn(ctx, nodeID)
	}
	return upgradepkg.AgentUpdateStatus{State: model.NodeUpgradeSucceeded}, nil
}

type upgraderClientFunc struct {
	upgradeFn func(context.Context, upgradepkg.PanelUpgradeRequest) error
}

func (f upgraderClientFunc) Status(context.Context) (*upgradepkg.UpgraderStatus, error) {
	return &upgradepkg.UpgraderStatus{}, nil
}
func (f upgraderClientFunc) Upgrade(ctx context.Context, req upgradepkg.PanelUpgradeRequest) error {
	if f.upgradeFn != nil {
		return f.upgradeFn(ctx, req)
	}
	return nil
}
func (f upgraderClientFunc) Rollback(context.Context, upgradepkg.RollbackRequest) error { return nil }
func (f upgraderClientFunc) Healthz(context.Context) error                              { return nil }

func codeOf(err error) string {
	var appErr *response.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

// ---- 状态 ----

func TestUpgradeStatusNilService(t *testing.T) {
	resp, err := upgrade.NewUpgradeStatusLogic(context.Background(), &svc.ServiceContext{}).UpgradeStatus()
	if err != nil || resp != nil {
		t.Fatalf("resp = %v, err = %v; want nil, nil", resp, err)
	}
}

func TestUpgradeStatusMapsLockAndActiveTask(t *testing.T) {
	expires := time.Now().UTC()
	svcCtx := &svc.ServiceContext{Upgrade: &upgradepkg.Service{
		PanelVersion: "1.2.3",
		Channel:      "stable",
		Model: upgradeModelFunc{
			getLockFn: func(context.Context) (*model.UpgradeLock, error) {
				return &model.UpgradeLock{Holder: "panel/x", Scope: "panel", ExpiresAt: expires}, nil
			},
			activeTaskFn: func(context.Context) (*model.UpgradeTask, error) {
				return &model.UpgradeTask{ID: "t1", Status: model.UpgradeTaskRunning}, nil
			},
		},
	}}

	resp, err := upgrade.NewUpgradeStatusLogic(context.Background(), svcCtx).UpgradeStatus()
	if err != nil {
		t.Fatalf("UpgradeStatus() error = %v", err)
	}
	if resp.Data.Panel.CurrentVersion != "1.2.3" || resp.Data.Lock == nil || resp.Data.Lock.Holder != "panel/x" {
		t.Fatalf("data = %+v", resp.Data)
	}
	if resp.Data.ActiveTask == nil || resp.Data.ActiveTask.Id != "t1" {
		t.Fatalf("activeTask = %+v", resp.Data.ActiveTask)
	}
}

func TestUpgradeStatusModelError(t *testing.T) {
	svcCtx := &svc.ServiceContext{Upgrade: &upgradepkg.Service{Model: upgradeModelFunc{
		getLockFn: func(context.Context) (*model.UpgradeLock, error) { return nil, errors.New("boom") },
	}}}
	_, err := upgrade.NewUpgradeStatusLogic(context.Background(), svcCtx).UpgradeStatus()
	if codeOf(err) != "INTERNAL" {
		t.Fatalf("code = %q, want INTERNAL", codeOf(err))
	}
}

// ---- 检查更新 ----

func TestUpgradeCheckNilService(t *testing.T) {
	resp, err := upgrade.NewUpgradeCheckLogic(context.Background(), &svc.ServiceContext{}).UpgradeCheck()
	if err != nil {
		t.Fatalf("UpgradeCheck() error = %v", err)
	}
	if resp.Data.ErrorCode != "UPSTREAM_UPDATE_SOURCE_FAILED" {
		t.Fatalf("errorCode = %q, want UPSTREAM_UPDATE_SOURCE_FAILED", resp.Data.ErrorCode)
	}
}

func TestUpgradeCheckSourceNotConfiguredIsNotFailure(t *testing.T) {
	svcCtx := &svc.ServiceContext{Upgrade: &upgradepkg.Service{PanelVersion: "1.0.0"}}
	resp, err := upgrade.NewUpgradeCheckLogic(context.Background(), svcCtx).UpgradeCheck()
	if err != nil {
		t.Fatalf("UpgradeCheck() error = %v", err)
	}
	if resp.Data.ErrorCode != "" {
		t.Fatalf("errorCode = %q, want empty (not configured is not a failure)", resp.Data.ErrorCode)
	}
	if resp.Data.CheckedAt == "" {
		t.Fatal("checkedAt must be populated")
	}
}

func TestUpgradeCheckSourceError(t *testing.T) {
	svcCtx := &svc.ServiceContext{Upgrade: &upgradepkg.Service{
		PanelVersion: "1.0.0",
		Source: manifestSourceFunc{fetchFn: func(context.Context, string) (*upgradepkg.ReleaseManifest, error) {
			return nil, errors.New("source down")
		}},
	}}
	resp, err := upgrade.NewUpgradeCheckLogic(context.Background(), svcCtx).UpgradeCheck()
	if err != nil {
		t.Fatalf("UpgradeCheck() error = %v", err)
	}
	if resp.Data.ErrorCode != "UPSTREAM_UPDATE_SOURCE_FAILED" {
		t.Fatalf("errorCode = %q, want UPSTREAM_UPDATE_SOURCE_FAILED", resp.Data.ErrorCode)
	}
}

func TestUpgradeCheckSuccess(t *testing.T) {
	svcCtx := &svc.ServiceContext{Upgrade: &upgradepkg.Service{
		PanelVersion: "1.0.0",
		Source: manifestSourceFunc{fetchFn: func(context.Context, string) (*upgradepkg.ReleaseManifest, error) {
			return &upgradepkg.ReleaseManifest{Version: "1.2.0"}, nil
		}},
	}}
	resp, err := upgrade.NewUpgradeCheckLogic(context.Background(), svcCtx).UpgradeCheck()
	if err != nil {
		t.Fatalf("UpgradeCheck() error = %v", err)
	}
	if !resp.Data.UpdateAvailable || resp.Data.LatestVersion != "1.2.0" {
		t.Fatalf("data = %+v, want update 1.2.0", resp.Data)
	}
}

// ---- 发起 Panel 升级 ----

func TestUpgradePanelMissingTarget(t *testing.T) {
	_, err := upgrade.NewUpgradePanelLogic(context.Background(), &svc.ServiceContext{Upgrade: &upgradepkg.Service{}}).UpgradePanel(&types.UpgradePanelReq{})
	if codeOf(err) != "VALIDATION_MISSING_FIELD" {
		t.Fatalf("code = %q, want VALIDATION_MISSING_FIELD", codeOf(err))
	}
}

func TestUpgradePanelServiceNil(t *testing.T) {
	_, err := upgrade.NewUpgradePanelLogic(context.Background(), &svc.ServiceContext{}).UpgradePanel(&types.UpgradePanelReq{TargetVersion: "1.1.0"})
	if codeOf(err) != "UPSTREAM_UPGRADER_UNAVAILABLE" {
		t.Fatalf("code = %q, want UPSTREAM_UPGRADER_UNAVAILABLE", codeOf(err))
	}
}

func TestUpgradePanelComposeManual(t *testing.T) {
	_, err := upgrade.NewUpgradePanelLogic(context.Background(), &svc.ServiceContext{
		Upgrade: &upgradepkg.Service{ComposeMode: true, PanelVersion: "1.0.0"},
	}).UpgradePanel(&types.UpgradePanelReq{TargetVersion: "1.1.0"})
	if codeOf(err) != "CONFLICT_COMPOSE_MANUAL_REQUIRED" {
		t.Fatalf("code = %q, want CONFLICT_COMPOSE_MANUAL_REQUIRED", codeOf(err))
	}
}

func TestUpgradePanelInvalidTarget(t *testing.T) {
	_, err := upgrade.NewUpgradePanelLogic(context.Background(), &svc.ServiceContext{
		Upgrade: &upgradepkg.Service{PanelVersion: "2.0.0"},
	}).UpgradePanel(&types.UpgradePanelReq{TargetVersion: "1.9.0"})
	if codeOf(err) != "CONFLICT_UPGRADE_TARGET_INVALID" {
		t.Fatalf("code = %q, want CONFLICT_UPGRADE_TARGET_INVALID", codeOf(err))
	}
}

func TestUpgradePanelSuccess(t *testing.T) {
	var created *model.UpgradeTask
	svcCtx := &svc.ServiceContext{Upgrade: &upgradepkg.Service{
		PanelVersion: "1.0.0",
		Upgrader:     upgraderClientFunc{},
		Model: upgradeModelFunc{
			createTaskFn: func(_ context.Context, task *model.UpgradeTask) error { created = task; return nil },
		},
	}}
	resp, err := upgrade.NewUpgradePanelLogic(context.Background(), svcCtx).UpgradePanel(&types.UpgradePanelReq{TargetVersion: "1.1.0", OperationId: "op-1"})
	if err != nil {
		t.Fatalf("UpgradePanel() error = %v", err)
	}
	if created == nil || resp.Data.Item.TargetVersion != "1.1.0" || resp.Data.Item.Id != created.ID {
		t.Fatalf("resp = %+v, created = %+v", resp.Data.Item, created)
	}
}

// ---- 待升级清单 ----

func TestUpgradePendingSuccess(t *testing.T) {
	now := time.Now().UTC()
	svcCtx := &svc.ServiceContext{UpgradeModel: upgradeModelFunc{
		listTargetsFn: func(context.Context) ([]model.UpgradeTarget, error) {
			return []model.UpgradeTarget{{NodeID: "n1", NodeName: "node-1", TargetVersion: "1.1.0", BatchID: "b1", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}}, nil
		},
	}}
	resp, err := upgrade.NewUpgradePendingLogic(context.Background(), svcCtx).UpgradePending()
	if err != nil {
		t.Fatalf("UpgradePending() error = %v", err)
	}
	if len(resp.Data.Items) != 1 || resp.Data.Items[0].NodeId != "n1" || resp.Data.Items[0].ExpiresAt == "" {
		t.Fatalf("items = %+v", resp.Data.Items)
	}
}

func TestUpgradePendingModelError(t *testing.T) {
	svcCtx := &svc.ServiceContext{UpgradeModel: upgradeModelFunc{
		listTargetsFn: func(context.Context) ([]model.UpgradeTarget, error) { return nil, errors.New("boom") },
	}}
	_, err := upgrade.NewUpgradePendingLogic(context.Background(), svcCtx).UpgradePending()
	if codeOf(err) != "INTERNAL" {
		t.Fatalf("code = %q, want INTERNAL", codeOf(err))
	}
}

// ---- 任务详情 ----

func TestUpgradeTaskDetailNotFound(t *testing.T) {
	svcCtx := &svc.ServiceContext{UpgradeModel: upgradeModelFunc{
		getTaskFn: func(context.Context, string) (*model.UpgradeTask, error) { return nil, model.ErrNotFound },
	}}
	_, err := upgrade.NewUpgradeTaskDetailLogic(context.Background(), svcCtx).UpgradeTaskDetail(&types.UpgradeTaskDetailReq{Id: "missing"})
	if codeOf(err) != "NOT_FOUND_UPGRADE_TASK" {
		t.Fatalf("code = %q, want NOT_FOUND_UPGRADE_TASK", codeOf(err))
	}
}

func TestUpgradeTaskDetailSuccessMapsNodes(t *testing.T) {
	now := time.Now().UTC()
	svcCtx := &svc.ServiceContext{UpgradeModel: upgradeModelFunc{
		getTaskFn: func(context.Context, string) (*model.UpgradeTask, error) {
			return &model.UpgradeTask{
				ID: "t1", Status: model.UpgradeTaskRunning, CreatedAt: now, StartedAt: &now,
				Nodes: []model.UpgradeTaskNode{{NodeID: "n1", State: model.NodeUpgradeApplying, UpdatedAt: now}},
			}, nil
		},
	}}
	resp, err := upgrade.NewUpgradeTaskDetailLogic(context.Background(), svcCtx).UpgradeTaskDetail(&types.UpgradeTaskDetailReq{Id: "t1"})
	if err != nil {
		t.Fatalf("UpgradeTaskDetail() error = %v", err)
	}
	if resp.Data.Item.Id != "t1" || len(resp.Data.Item.Nodes) != 1 || resp.Data.Item.Nodes[0].NodeId != "n1" {
		t.Fatalf("item = %+v", resp.Data.Item)
	}
	if resp.Data.Item.StartedAt == "" {
		t.Fatal("startedAt must be populated")
	}
}

// ---- 任务列表 ----

func TestUpgradeTaskListSuccess(t *testing.T) {
	var gotFilter model.UpgradeTaskFilter
	svcCtx := &svc.ServiceContext{UpgradeModel: upgradeModelFunc{
		listTasksFn: func(_ context.Context, f model.UpgradeTaskFilter) ([]model.UpgradeTask, int64, error) {
			gotFilter = f
			return []model.UpgradeTask{{ID: "t1", Status: model.UpgradeTaskSucceeded, CreatedAt: time.Now().UTC()}}, 1, nil
		},
	}}
	resp, err := upgrade.NewUpgradeTaskListLogic(context.Background(), svcCtx).UpgradeTaskList(&types.UpgradeTaskListReq{Page: 2, PageSize: 5, Scope: "panel", Status: "succeeded"})
	if err != nil {
		t.Fatalf("UpgradeTaskList() error = %v", err)
	}
	if gotFilter.Page != 2 || gotFilter.PageSize != 5 || gotFilter.Scope != "panel" || gotFilter.Status != "succeeded" {
		t.Fatalf("filter = %+v", gotFilter)
	}
	if resp.Data.Total != 1 || resp.Data.Page != 2 || resp.Data.PageSize != 5 || len(resp.Data.Items) != 1 {
		t.Fatalf("data = %+v", resp.Data)
	}
}

func TestUpgradeTaskListModelError(t *testing.T) {
	svcCtx := &svc.ServiceContext{UpgradeModel: upgradeModelFunc{
		listTasksFn: func(context.Context, model.UpgradeTaskFilter) ([]model.UpgradeTask, int64, error) {
			return nil, 0, errors.New("boom")
		},
	}}
	_, err := upgrade.NewUpgradeTaskListLogic(context.Background(), svcCtx).UpgradeTaskList(&types.UpgradeTaskListReq{})
	if codeOf(err) != "INTERNAL" {
		t.Fatalf("code = %q, want INTERNAL", codeOf(err))
	}
}

// ---- 重试 ----

func TestUpgradeTaskRetryServiceNil(t *testing.T) {
	_, err := upgrade.NewUpgradeTaskRetryLogic(context.Background(), &svc.ServiceContext{}).UpgradeTaskRetry(&types.UpgradeTaskRetryReq{Id: "t1"})
	if codeOf(err) != "UPSTREAM_UPGRADER_UNAVAILABLE" {
		t.Fatalf("code = %q, want UPSTREAM_UPGRADER_UNAVAILABLE", codeOf(err))
	}
}

func TestUpgradeTaskRetryNotRetryable(t *testing.T) {
	svcCtx := &svc.ServiceContext{Upgrade: &upgradepkg.Service{Model: upgradeModelFunc{
		getTaskFn: func(context.Context, string) (*model.UpgradeTask, error) {
			return &model.UpgradeTask{ID: "t1", Status: model.UpgradeTaskRunning}, nil
		},
	}}}
	_, err := upgrade.NewUpgradeTaskRetryLogic(context.Background(), svcCtx).UpgradeTaskRetry(&types.UpgradeTaskRetryReq{Id: "t1"})
	if codeOf(err) != "CONFLICT_UPGRADE_TARGET_INVALID" {
		t.Fatalf("code = %q, want CONFLICT_UPGRADE_TARGET_INVALID", codeOf(err))
	}
}

func TestUpgradeTaskRetryPanelSuccess(t *testing.T) {
	svcCtx := &svc.ServiceContext{Upgrade: &upgradepkg.Service{
		PanelVersion: "1.0.0",
		Upgrader:     upgraderClientFunc{},
		Model: upgradeModelFunc{
			getTaskFn: func(context.Context, string) (*model.UpgradeTask, error) {
				return &model.UpgradeTask{ID: "t1", Scope: "panel", Status: model.UpgradeTaskFailed, TargetVersion: "1.1.0"}, nil
			},
		},
	}}
	resp, err := upgrade.NewUpgradeTaskRetryLogic(context.Background(), svcCtx).UpgradeTaskRetry(&types.UpgradeTaskRetryReq{Id: "t1"})
	if err != nil {
		t.Fatalf("UpgradeTaskRetry() error = %v", err)
	}
	if resp.Data.Item.TargetVersion != "1.1.0" || resp.Data.Item.Id == "" {
		t.Fatalf("item = %+v", resp.Data.Item)
	}
}

// ---- 发起 Agent 升级 ----

func TestUpgradeAgentsValidation(t *testing.T) {
	l := upgrade.NewUpgradeAgentsLogic(context.Background(), &svc.ServiceContext{})
	cases := []struct {
		name string
		req  *types.UpgradeAgentReq
		code string
	}{
		{"missing-target", &types.UpgradeAgentReq{NodeIds: []string{"n1"}}, "VALIDATION_MISSING_FIELD"},
		{"both-groups", &types.UpgradeAgentReq{TargetVersion: "1.1.0", NodeIds: []string{"n1"}, GroupId: "g1"}, "VALIDATION_INVALID_FIELD"},
		{"neither", &types.UpgradeAgentReq{TargetVersion: "1.1.0"}, "VALIDATION_INVALID_FIELD"},
		{"bad-group-type", &types.UpgradeAgentReq{TargetVersion: "1.1.0", GroupId: "g1", GroupType: "region"}, "VALIDATION_INVALID_FIELD"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.UpgradeAgents(tc.req); codeOf(err) != tc.code {
				t.Fatalf("code = %q, want %s", codeOf(err), tc.code)
			}
		})
	}
}

func TestUpgradeAgentsServiceNil(t *testing.T) {
	_, err := upgrade.NewUpgradeAgentsLogic(context.Background(), &svc.ServiceContext{}).UpgradeAgents(&types.UpgradeAgentReq{
		TargetVersion: "1.1.0", NodeIds: []string{"n1"},
	})
	if codeOf(err) != "UPSTREAM_AGENT_UNAVAILABLE" {
		t.Fatalf("code = %q, want UPSTREAM_AGENT_UNAVAILABLE", codeOf(err))
	}
}

func TestUpgradeAgentsSuccessRunsBatch(t *testing.T) {
	now := time.Now().UTC()
	var updated int
	svcCtx := &svc.ServiceContext{
		NodeModel: nodeModelForUpgrade{getFn: func(context.Context, string) (*model.NodeDetail, error) {
			return &model.NodeDetail{NodeSummary: model.NodeSummary{
				ID: "n1", Status: "online", LastSeenAt: &now, AgentVersion: "1.0.0",
			}}, nil
		}},
		Upgrade: &upgradepkg.Service{
			PanelVersion: "1.0.0",
			PanelMinor:   "4",
			AgentMinor:   "4",
			SchemaEqual:  true,
			PollInterval: time.Millisecond,
			PollAttempts: 1,
			Agents:       agentUpdaterFunc{},
			Nodes: nodeModelForUpgrade{getFn: func(context.Context, string) (*model.NodeDetail, error) {
				return &model.NodeDetail{NodeSummary: model.NodeSummary{
					ID: "n1", Status: "online", LastSeenAt: &now, AgentVersion: "1.0.0",
				}}, nil
			}},
			Model: upgradeModelFunc{
				updateTaskStatusFn: func(_ context.Context, id, status, code, msg string, _ *time.Time, _ *time.Time) error {
					if status == model.UpgradeTaskSucceeded {
						updated++
					}
					return nil
				},
				getTaskFn: func(context.Context, string) (*model.UpgradeTask, error) {
					return &model.UpgradeTask{ID: "t1", TargetVersion: "1.1.0", Status: model.UpgradeTaskSucceeded}, nil
				},
			},
		},
	}
	resp, err := upgrade.NewUpgradeAgentsLogic(context.Background(), svcCtx).UpgradeAgents(&types.UpgradeAgentReq{
		TargetVersion: "1.1.0", NodeIds: []string{"n1"},
	})
	if err != nil {
		t.Fatalf("UpgradeAgents() error = %v", err)
	}
	if resp.Data.Item.TargetVersion != "1.1.0" || updated != 1 {
		t.Fatalf("item = %+v, updated = %d", resp.Data.Item, updated)
	}
}

func TestUpgradeAgentsCheckTargetInvalid(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		NodeModel: nodeModelForUpgrade{},
		Upgrade: &upgradepkg.Service{
			PanelVersion: "1.0.0",
			Agents:       agentUpdaterFunc{},
		},
	}
	_, err := upgrade.NewUpgradeAgentsLogic(context.Background(), svcCtx).UpgradeAgents(&types.UpgradeAgentReq{
		TargetVersion: "0.9.0", NodeIds: []string{"n1"},
	})
	if codeOf(err) != "CONFLICT_UPGRADE_TARGET_INVALID" {
		t.Fatalf("code = %q, want CONFLICT_UPGRADE_TARGET_INVALID", codeOf(err))
	}
}

func TestUpgradeAgentsLockBusy(t *testing.T) {
	now := time.Now().UTC()
	svcCtx := &svc.ServiceContext{
		NodeModel: nodeModelForUpgrade{getFn: func(context.Context, string) (*model.NodeDetail, error) {
			return &model.NodeDetail{NodeSummary: model.NodeSummary{ID: "n1", Status: "online", LastSeenAt: &now}}, nil
		}},
		Upgrade: &upgradepkg.Service{
			PanelVersion: "1.0.0",
			PanelMinor:   "4",
			AgentMinor:   "4",
			SchemaEqual:  true,
			Agents:       agentUpdaterFunc{},
			Model: upgradeModelFunc{
				acquireLockFn: func(context.Context, string, string, time.Time, time.Duration) (bool, error) {
					return false, nil
				},
			},
		},
	}
	_, err := upgrade.NewUpgradeAgentsLogic(context.Background(), svcCtx).UpgradeAgents(&types.UpgradeAgentReq{
		TargetVersion: "1.1.0", NodeIds: []string{"n1"},
	})
	if codeOf(err) != "OPERATION_IN_PROGRESS" {
		t.Fatalf("code = %q, want OPERATION_IN_PROGRESS", codeOf(err))
	}
}
