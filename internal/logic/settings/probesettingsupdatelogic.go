// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type ProbeSettingsUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 保存探测规格并触发下发
func NewProbeSettingsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProbeSettingsUpdateLogic {
	return &ProbeSettingsUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProbeSettingsUpdateLogic) ProbeSettingsUpdate(req *types.ProbeSettingsUpdateReq) (resp *types.ProbeSettingsResp, err error) {
	if req.FrequencySeconds < 5 || req.FrequencySeconds > 3600 {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "frequencySeconds", "must be between 5 and 3600")
	}
	if req.TimeoutMs < 100 || req.TimeoutMs > 60000 {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "timeoutMs", "must be between 100 and 60000")
	}
	if req.FailureThreshold < 1 || req.FailureThreshold > 10 {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "failureThreshold", "must be between 1 and 10")
	}
	if req.RecoveryThreshold < 1 || req.RecoveryThreshold > 10 {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "recoveryThreshold", "must be between 1 and 10")
	}

	settings := probeSettings{
		FrequencySeconds:  req.FrequencySeconds,
		TimeoutMs:         req.TimeoutMs,
		FailureThreshold:  req.FailureThreshold,
		RecoveryThreshold: req.RecoveryThreshold,
	}
	if err := saveProbeSettings(l.ctx, l.svcCtx.SettingModel, settings); err != nil {
		return nil, response.Internal(err)
	}

	_ = l.createApplyTask()

	l.writeAudit()

	return &types.ProbeSettingsResp{
		Code: errcode.OK,
		Data: types.ProbeSettingsData{
			FrequencySeconds:  settings.FrequencySeconds,
			TimeoutMs:         settings.TimeoutMs,
			FailureThreshold:  settings.FailureThreshold,
			RecoveryThreshold: settings.RecoveryThreshold,
			UpdatedAt:         time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}

// createApplyTask 保存后创建下发任务；模型 / 节点不可用时只落配置，不影响保存。
func (l *ProbeSettingsUpdateLogic) createApplyTask() string {
	if l.svcCtx.SettingsApplyModel == nil {
		return ""
	}
	now := time.Now().UTC()
	taskID := uuid.NewString()
	if err := l.svcCtx.SettingsApplyModel.CreateTask(l.ctx, &model.SettingsApplyTask{
		ID: taskID, Kind: "probe", State: "queued",
		CreatedBy: l.actor(), RequestID: requestid.FromContext(l.ctx), CreatedAt: now,
	}); err != nil {
		l.Errorw("settings.probe.apply_task", logx.Field("error", err.Error()))
		return ""
	}
	if l.svcCtx.NodeModel == nil {
		return taskID
	}
	nodes, _, err := l.svcCtx.NodeModel.List(l.ctx, model.NodeFilter{Page: 1, PageSize: 100})
	if err != nil {
		return taskID
	}
	for _, n := range nodes {
		_ = l.svcCtx.SettingsApplyModel.UpsertNode(l.ctx, &model.SettingsApplyNode{
			TaskID: taskID, NodeID: n.ID, State: model.NodeUpgradePending, UpdatedAt: now,
		})
	}
	return taskID
}

func (l *ProbeSettingsUpdateLogic) actor() string {
	if row := auth.SessionFromContext(l.ctx); row != nil {
		return row.User.Account
	}
	return ""
}

func (l *ProbeSettingsUpdateLogic) writeAudit() {
	if l.svcCtx.AuditLogModel == nil {
		return
	}
	_ = l.svcCtx.AuditLogModel.Insert(l.ctx, model.AuditLog{
		Actor: l.actor(), Action: "settings.probe.update", ObjectType: "settings", ObjectID: "probe",
		Result: "success", RequestID: requestid.FromContext(l.ctx), CreatedAt: time.Now().UTC(),
	})
}
