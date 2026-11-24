package upgrade

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/relayx-tech/panel/internal/model"
)

var (
	ErrLockBusy            = errors.New("another upgrade is in progress")
	ErrComposeManual       = errors.New("docker compose deployment requires manual rebuild")
	ErrUpgraderUnavailable = errors.New("upgrader is unavailable")
	ErrAgentUnavailable    = errors.New("agent is unavailable")
	ErrLockLost            = errors.New("upgrade lock lost")
	ErrTaskNotRetryable    = errors.New("task is not retryable")
)

const (
	PanelScope       = "panel"
	AgentBatchScope  = "agent_batch"
	AgentBatchTTL    = 2 * time.Hour
	PanelUpgradeTTL  = 30 * time.Minute
	offlineThreshold = 5 * time.Minute
	pendingTTL       = 30 * 24 * time.Hour
)

// AgentUpdateRequest 是 Panel 对 Agent 的升级下发。
type AgentUpdateRequest struct {
	NodeID        string
	OperationID   string
	TargetVersion string
	SourceURL     string
	SHA256        string
	Signature     string
	Reason        string // upgrade | rollback
	Deadline      time.Duration
}

type AgentUpdateResult struct {
	Accepted       bool
	State          string
	CurrentVersion string
	ErrorCode      string
}

type AgentUpdateStatus struct {
	State         string
	TargetVersion string
	ErrorCode     string
	ErrorMessage  string
}

// AgentUpdater 抽象 Panel → Agent 的升级通道（生产实现走 gRPC Update）。
type AgentUpdater interface {
	Update(ctx context.Context, req AgentUpdateRequest) (AgentUpdateResult, error)
	Status(ctx context.Context, nodeID string) (AgentUpdateStatus, error)
}

// ReleaseManifest 是更新清单（已验签）。
type ReleaseManifest struct {
	Version     string
	Channel     string
	Assets      []Asset
	ManifestURL string
	Signature   string
	PublishedAt time.Time
}

// ManifestSource 拉取并验签更新清单。
type ManifestSource interface {
	Fetch(ctx context.Context, channel string) (*ReleaseManifest, error)
}

// AlertDispatcher 在告警落库后投递通知渠道。
type AlertDispatcher interface {
	Dispatch(ctx context.Context, alertID int64, level, message string)
}

// Service 是升级编排（design.md §5.5）。
type Service struct {
	Model    model.UpgradeModel
	Nodes    model.NodeModel
	Alerts   model.AlertModel
	Agents   AgentUpdater
	Upgrader UpgraderClient
	Source   ManifestSource
	Notifier AlertDispatcher

	Holder       string
	PanelVersion string
	Channel      string
	ComposeMode  bool
	SchemaEqual  bool

	PanelMinor string
	AgentMinor string

	PanelTTL     time.Duration
	AgentTTL     time.Duration
	PollInterval time.Duration
	PollAttempts int
	Async        bool
	Now          func() time.Time

	mu         sync.RWMutex
	latest     string
	agentAsset Asset
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) holder() string {
	if s.Holder != "" {
		return s.Holder
	}
	return "panel/" + uuid.NewString()
}

func (s *Service) panelTTL() time.Duration {
	if s.PanelTTL > 0 {
		return s.PanelTTL
	}
	return PanelUpgradeTTL
}

func (s *Service) agentTTL() time.Duration {
	if s.AgentTTL > 0 {
		return s.AgentTTL
	}
	return AgentBatchTTL
}

func (s *Service) Version() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latest
}

func (s *Service) setVersion(v string) {
	s.mu.Lock()
	s.latest = v
	s.mu.Unlock()
}

// setAgentAsset 从清单中选择 Agent 升级资产（名称包含 agent）。
func (s *Service) setAgentAsset(assets []Asset) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range assets {
		if strings.Contains(strings.ToLower(a.Name), "agent") {
			s.agentAsset = a
			return
		}
	}
}

func (s *Service) agentAssetInfo() Asset {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agentAsset
}

// StatusView 是 /upgrade/status 的领域视图。
type StatusView struct {
	Current string
	Latest  string
	Channel string
	Compose bool
	Lock    *model.UpgradeLock
	Active  *model.UpgradeTask
}

func (s *Service) Status(ctx context.Context) (*StatusView, error) {
	lock, err := s.Model.GetLock(ctx)
	if err != nil {
		return nil, err
	}
	active, err := s.Model.ActiveTask(ctx)
	if err != nil {
		return nil, err
	}
	return &StatusView{
		Current: s.PanelVersion, Latest: s.Version(), Channel: s.Channel,
		Compose: s.ComposeMode, Lock: lock, Active: active,
	}, nil
}

// CheckResult 是 /upgrade/check 的结果。
type CheckResult struct {
	CheckedAt       time.Time
	LatestVersion   string
	UpdateAvailable bool
}

// Check 拉取更新清单；失败时保留上次结果。
func (s *Service) Check(ctx context.Context) (*CheckResult, error) {
	if s.Source == nil {
		return nil, ErrUpgraderUnavailable
	}
	manifest, err := s.Source.Fetch(ctx, s.Channel)
	if err != nil {
		return &CheckResult{CheckedAt: s.now(), LatestVersion: s.Version()}, err
	}
	s.setVersion(manifest.Version)
	s.setAgentAsset(manifest.Assets)
	available := false
	if cur, err := ParseVersion(s.PanelVersion); err == nil {
		if next, err := ParseVersion(manifest.Version); err == nil {
			available = next.Compare(cur) > 0
		}
	}
	return &CheckResult{CheckedAt: s.now(), LatestVersion: manifest.Version, UpdateAvailable: available}, nil
}

// StartPanel 发起 Panel 自升级；返回 queued 任务（由 upgrader 异步执行）。
func (s *Service) StartPanel(ctx context.Context, actor, requestID, operationID, target string) (*model.UpgradeTask, error) {
	if s.ComposeMode {
		return nil, ErrComposeManual
	}
	if err := CheckTarget(s.PanelVersion, target); err != nil {
		return nil, err
	}
	if s.Upgrader == nil {
		return nil, ErrUpgraderUnavailable
	}
	if err := s.acquire(ctx, PanelScope, s.panelTTL()); err != nil {
		return nil, err
	}
	now := s.now()
	task := &model.UpgradeTask{
		ID: uuid.NewString(), Scope: PanelScope, TargetVersion: target, FromVersion: s.PanelVersion,
		Status: model.UpgradeTaskRunning, CreatedBy: actor, RequestID: requestID, CreatedAt: now, StartedAt: &now,
	}
	if err := s.Model.CreateTask(ctx, task); err != nil {
		s.release(ctx)
		return nil, err
	}
	req := PanelUpgradeRequest{OperationID: operationID, TargetVersion: target, DeadlineSeconds: int(s.panelTTL().Seconds())}
	if s.Source != nil {
		if manifest, err := s.Source.Fetch(ctx, s.Channel); err == nil {
			req.ManifestURL = manifest.ManifestURL
			req.ManifestSignature = manifest.Signature
			req.Assets = manifest.Assets
		}
	}
	if err := s.Upgrader.Upgrade(ctx, req); err != nil {
		_ = s.Model.UpdateTaskStatus(ctx, task.ID, model.UpgradeTaskFailed, "UPSTREAM_UPGRADER_UNAVAILABLE", err.Error(), nil, &now)
		task.Status = model.UpgradeTaskFailed
		s.release(ctx)
		s.raise(ctx, "critical", "upgrade_failed", "upgrade_task", task.ID, "panel upgrade failed to start: "+err.Error())
		return nil, ErrUpgraderUnavailable
	}
	return task, nil
}

// StartAgents 发起 Agent 单节点 / 批次升级。
func (s *Service) StartAgents(ctx context.Context, actor, requestID, operationID, target string, nodeIDs []string, groupID, groupType string) (*model.UpgradeTask, error) {
	if (len(nodeIDs) > 0) == (groupID != "") {
		return nil, errors.New("nodeIds and groupId are mutually exclusive and one is required")
	}
	if s.Agents == nil {
		return nil, ErrAgentUnavailable
	}
	if err := s.CheckAgentTarget(target); err != nil {
		return nil, err
	}
	if err := s.acquire(ctx, AgentBatchScope, s.agentTTL()); err != nil {
		return nil, err
	}

	nodes, err := s.resolveNodes(ctx, nodeIDs, groupID, groupType)
	if err != nil {
		s.release(ctx)
		return nil, err
	}
	now := s.now()
	task := &model.UpgradeTask{
		ID: uuid.NewString(), Scope: AgentBatchScope, TargetVersion: target,
		Status: model.UpgradeTaskRunning, CreatedBy: actor, RequestID: requestID, CreatedAt: now, StartedAt: &now,
	}
	if err := s.Model.CreateTask(ctx, task); err != nil {
		s.release(ctx)
		return nil, err
	}
	online := make([]model.NodeSummary, 0, len(nodes))
	for _, n := range nodes {
		if s.isOnline(n) {
			online = append(online, n)
			continue
		}
		_ = s.Model.UpsertTarget(ctx, model.UpgradeTarget{
			NodeID: n.ID, TargetVersion: target, BatchID: task.ID, CreatedAt: now, ExpiresAt: now.Add(pendingTTL),
		})
	}
	if len(online) == 0 {
		_ = s.Model.UpdateTaskStatus(ctx, task.ID, model.UpgradeTaskSucceeded, "", "", nil, &now)
		task.Status = model.UpgradeTaskSucceeded
		s.release(ctx)
		return task, nil
	}
	for _, n := range online {
		task.FromVersion = firstNonEmpty(task.FromVersion, n.AgentVersion)
		_ = s.Model.UpsertTaskNode(ctx, &model.UpgradeTaskNode{
			TaskID: task.ID, NodeID: n.ID, FromVersion: n.AgentVersion, ToVersion: target,
			State: model.NodeUpgradePending, UpdatedAt: now,
		})
	}

	if s.Async {
		go func(ctx context.Context, task *model.UpgradeTask, nodes []model.NodeSummary, operationID string) {
			_ = s.runBatch(ctx, task, nodes, operationID)
		}(context.Background(), task, online, operationID)
		return task, nil
	}
	if err := s.runBatch(ctx, task, online, operationID); err != nil {
		return task, nil
	}
	full, err := s.Model.GetTask(ctx, task.ID)
	if err != nil {
		return task, nil
	}
	return full, nil
}

func (s *Service) runBatch(ctx context.Context, task *model.UpgradeTask, nodes []model.NodeSummary, operationID string) error {
	succeeded := make([]model.NodeSummary, 0, len(nodes))
	for _, node := range nodes {
		state, code, err := s.upgradeNode(ctx, task, node, operationID)
		now := s.now()
		switch {
		case err == nil && state == model.NodeUpgradeSucceeded:
			succeeded = append(succeeded, node)
			_ = s.Model.UpsertTaskNode(ctx, &model.UpgradeTaskNode{
				TaskID: task.ID, NodeID: node.ID, FromVersion: node.AgentVersion, ToVersion: task.TargetVersion,
				State: model.NodeUpgradeSucceeded, UpdatedAt: now,
			})
		case err == nil && state == model.NodeUpgradeRolledBack:
			_ = s.Model.UpsertTaskNode(ctx, &model.UpgradeTaskNode{
				TaskID: task.ID, NodeID: node.ID, FromVersion: node.AgentVersion, ToVersion: task.TargetVersion,
				State: model.NodeUpgradeRolledBack, ErrorCode: code, UpdatedAt: now,
			})
			return s.rollbackBatch(ctx, task, succeeded, code)
		default:
			_ = s.Model.UpsertTaskNode(ctx, &model.UpgradeTaskNode{
				TaskID: task.ID, NodeID: node.ID, FromVersion: node.AgentVersion, ToVersion: task.TargetVersion,
				State: model.NodeUpgradeFailed, ErrorCode: code, UpdatedAt: now,
			})
			s.raise(ctx, "warning", "upgrade_failed", "node", node.ID, "agent upgrade failed: "+code)
			return s.rollbackBatch(ctx, task, succeeded, code)
		}
	}
	now := s.now()
	_ = s.Model.UpdateTaskStatus(ctx, task.ID, model.UpgradeTaskSucceeded, "", "", nil, &now)
	s.release(ctx)
	return nil
}

func (s *Service) upgradeNode(ctx context.Context, task *model.UpgradeTask, node model.NodeSummary, operationID string) (string, string, error) {
	_ = s.Model.UpsertTaskNode(ctx, &model.UpgradeTaskNode{
		TaskID: task.ID, NodeID: node.ID, FromVersion: node.AgentVersion, ToVersion: task.TargetVersion,
		State: model.NodeUpgradeDownloading, UpdatedAt: s.now(),
	})
	res, err := s.Agents.Update(ctx, AgentUpdateRequest{
		NodeID: node.ID, OperationID: operationID, TargetVersion: task.TargetVersion, Reason: "upgrade",
		SourceURL: s.agentAssetInfo().URL, SHA256: s.agentAssetInfo().SHA256,
		Signature: s.agentAssetInfo().Signature, Deadline: s.agentTTL(),
	})
	if err != nil || !res.Accepted {
		if err != nil {
			return "", "UPSTREAM_AGENT_UNAVAILABLE", err
		}
		return model.NodeUpgradeFailed, res.ErrorCode, nil
	}
	return s.waitNode(ctx, node.ID)
}

func (s *Service) waitNode(ctx context.Context, nodeID string) (string, string, error) {
	interval := s.PollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	attempts := s.PollAttempts
	if attempts <= 0 {
		attempts = 150 // 5 分钟
	}
	for i := 0; i < attempts; i++ {
		status, err := s.Agents.Status(ctx, nodeID)
		if err == nil {
			switch status.State {
			case model.NodeUpgradeSucceeded:
				return model.NodeUpgradeSucceeded, "", nil
			case model.NodeUpgradeFailed:
				return model.NodeUpgradeFailed, firstNonEmpty(status.ErrorCode, "UPGRADE_FAILED"), nil
			case model.NodeUpgradeRolledBack:
				return model.NodeUpgradeRolledBack, firstNonEmpty(status.ErrorCode, "UPGRADE_ROLLED_BACK"), nil
			}
		}
		select {
		case <-ctx.Done():
			return model.NodeUpgradeFailed, "CONTEXT_CANCELLED", ctx.Err()
		case <-time.After(interval):
		}
	}
	return model.NodeUpgradeFailed, "UPGRADE_TIMEOUT", nil
}

func (s *Service) rollbackBatch(ctx context.Context, task *model.UpgradeTask, succeeded []model.NodeSummary, cause string) error {
	_ = s.Model.UpdateTaskStatus(ctx, task.ID, model.UpgradeTaskRollingBack, cause, "", nil, nil)
	for i := len(succeeded) - 1; i >= 0; i-- {
		node := succeeded[i]
		_, _ = s.Agents.Update(ctx, AgentUpdateRequest{
			NodeID: node.ID, OperationID: task.ID + "-rollback", TargetVersion: node.AgentVersion,
			Reason: "rollback", Deadline: s.agentTTL(),
		})
		_ = s.Model.UpsertTaskNode(ctx, &model.UpgradeTaskNode{
			TaskID: task.ID, NodeID: node.ID, FromVersion: task.TargetVersion, ToVersion: node.AgentVersion,
			State: model.NodeUpgradeRolledBack, UpdatedAt: s.now(),
		})
	}
	now := s.now()
	_ = s.Model.UpdateTaskStatus(ctx, task.ID, model.UpgradeTaskRolledBack, cause, "", nil, &now)
	_ = s.Model.DeleteTargetsByBatch(ctx, task.ID)
	s.release(ctx)
	s.raise(ctx, "critical", "upgrade_rolled_back", "upgrade_task", task.ID, "agent batch rolled back: "+cause)
	return nil
}

func (s *Service) resolveNodes(ctx context.Context, nodeIDs []string, groupID, groupType string) ([]model.NodeSummary, error) {
	if len(nodeIDs) > 0 {
		out := make([]model.NodeSummary, 0, len(nodeIDs))
		for _, id := range nodeIDs {
			detail, err := s.Nodes.Get(ctx, id)
			if err != nil {
				return nil, err
			}
			out = append(out, detail.NodeSummary)
		}
		return out, nil
	}
	items, _, err := s.Nodes.List(ctx, model.NodeFilter{Page: 1, PageSize: 100, Type: groupType, GroupID: groupID})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) isOnline(n model.NodeSummary) bool {
	if n.Status != "online" || n.LastSeenAt == nil {
		return false
	}
	return s.now().Sub(*n.LastSeenAt) <= offlineThreshold
}

// CheckAgentTarget 校验 Agent 升级目标（contract major / minor / schema）。
func (s *Service) CheckAgentTarget(target string) error {
	if err := CheckTarget(s.PanelVersion, target); err != nil {
		return err
	}
	return CheckCompatibility(ContractMajor, ContractMajor, s.PanelMinor, s.AgentMinor, s.SchemaEqual)
}

// Retry 重启失败 / 已回滚任务（生成新 operationId 与批次）。
func (s *Service) Retry(ctx context.Context, actor, requestID, taskID string) (*model.UpgradeTask, error) {
	task, err := s.Model.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Status != model.UpgradeTaskFailed && task.Status != model.UpgradeTaskRolledBack {
		return nil, ErrTaskNotRetryable
	}
	if task.Scope == PanelScope {
		return s.StartPanel(ctx, actor, requestID, uuid.NewString(), task.TargetVersion)
	}
	nodeIDs := make([]string, 0, len(task.Nodes))
	for _, n := range task.Nodes {
		nodeIDs = append(nodeIDs, n.NodeID)
	}
	return s.StartAgents(ctx, actor, requestID, uuid.NewString(), task.TargetVersion, nodeIDs, "", "")
}

func (s *Service) Pending(ctx context.Context) ([]model.UpgradeTarget, error) {
	return s.Model.ListTargets(ctx)
}

// OnNodeOnline 节点上线对账：仅批次 running/succeeded 且锁空闲时自动升级。
func (s *Service) OnNodeOnline(ctx context.Context, nodeID string, actor, requestID string) error {
	targets, err := s.Model.ListTargets(ctx)
	if err != nil {
		return err
	}
	var target *model.UpgradeTarget
	for i := range targets {
		if targets[i].NodeID == nodeID {
			target = &targets[i]
			break
		}
	}
	if target == nil {
		return nil
	}
	if target.ExpiresAt.Before(s.now()) {
		return s.Model.DeleteTarget(ctx, nodeID)
	}
	task, err := s.Model.GetTask(ctx, target.BatchID)
	if err != nil {
		return err
	}
	if task.Status == model.UpgradeTaskRolledBack || task.Status == model.UpgradeTaskAborted || task.Status == model.UpgradeTaskRollingBack {
		return s.Model.DeleteTarget(ctx, nodeID)
	}
	lock, err := s.Model.GetLock(ctx)
	if err != nil {
		return err
	}
	if lock != nil && lock.ExpiresAt.After(s.now()) {
		return nil
	}
	if _, err := s.StartAgents(ctx, actor, requestID, uuid.NewString(), target.TargetVersion, []string{nodeID}, "", ""); err != nil {
		return err
	}
	return s.Model.DeleteTarget(ctx, nodeID)
}

// Reconcile 启动时对账：Panel 任务终态回填、锁过期释放。
func (s *Service) Reconcile(ctx context.Context) error {
	if s.Upgrader == nil {
		return nil
	}
	active, err := s.Model.ActiveTask(ctx)
	if err != nil {
		return err
	}
	if active != nil && active.Scope == PanelScope {
		status, err := s.Upgrader.Status(ctx)
		if err == nil {
			now := s.now()
			switch status.State {
			case "succeeded":
				_ = s.Model.UpdateTaskStatus(ctx, active.ID, model.UpgradeTaskSucceeded, "", "", nil, &now)
				s.release(ctx)
			case "rolled_back", "failed":
				code := "UPGRADE_FAILED"
				message := ""
				if status.Error != nil {
					code = status.Error.Code
					message = status.Error.Message
				}
				_ = s.Model.UpdateTaskStatus(ctx, active.ID, status.State, code, message, nil, &now)
				s.release(ctx)
				s.raise(ctx, "critical", "upgrade_failed", "upgrade_task", active.ID, "panel upgrade "+status.State+": "+message)
			}
		}
	}
	lock, err := s.Model.GetLock(ctx)
	if err != nil {
		return err
	}
	if lock != nil && lock.ExpiresAt.Before(s.now()) {
		if active != nil {
			now := s.now()
			_ = s.Model.UpdateTaskStatus(ctx, active.ID, model.UpgradeTaskFailed, "UPGRADE_LOCK_EXPIRED", "lock expired", nil, &now)
		}
		return s.Model.ReleaseLock(ctx, lock.Holder)
	}
	return nil
}

func (s *Service) acquire(ctx context.Context, scope string, ttl time.Duration) error {
	ok, err := s.Model.AcquireLock(ctx, s.holder(), scope, s.now(), ttl)
	if err != nil {
		return err
	}
	if !ok {
		return ErrLockBusy
	}
	return nil
}

func (s *Service) release(ctx context.Context) {
	_ = s.Model.ReleaseLock(ctx, s.holder())
}

func (s *Service) raise(ctx context.Context, level, typ, objectType, objectID, message string) {
	if s.Alerts == nil {
		return
	}
	now := s.now()
	alert, err := s.Alerts.Upsert(ctx, model.Alert{
		Type: typ, Level: level, ObjectType: objectType, ObjectID: objectID,
		Message: message, Status: model.AlertStatusActive, FirstSeenAt: now, LastSeenAt: now,
	})
	if err == nil && alert != nil && s.Notifier != nil {
		s.Notifier.Dispatch(ctx, alert.ID, level, message)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
