// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package system

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	agentv1 "github.com/relayx-tech/contracts/gen/go/agent/v1"
	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/logic/shared"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type SystemHealthLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 健康检查
func NewSystemHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SystemHealthLogic {
	return &SystemHealthLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SystemHealthLogic) SystemHealth() (resp *types.SystemHealthResp, err error) {
	database := l.databaseItem()
	redis := l.redisItem()
	items := []types.HealthItem{
		{Key: "healthz", Status: "ok", Detail: "ok"},
		readyzItem(database, redis),
		database,
		redis,
		l.coreControlItem(),
		l.registrationItem(),
		l.agentItem(),
	}

	return &types.SystemHealthResp{
		Code:      errcode.OK,
		Data:      types.SystemHealthData{Items: items, CheckedAt: shared.TimeString(time.Now().UTC())},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}

// readyzItem 汇总就绪状态：数据库与 Redis 均可用才算就绪（AC85）。
func readyzItem(database, redis types.HealthItem) types.HealthItem {
	if database.Status == "ok" && redis.Status == "ok" {
		return types.HealthItem{Key: "readyz", Status: "ok", Detail: "dependencies ready"}
	}
	return types.HealthItem{Key: "readyz", Status: "degraded", Detail: "dependency unavailable"}
}

func (l *SystemHealthLogic) redisItem() types.HealthItem {
	if l.svcCtx.Redis == nil {
		return types.HealthItem{Key: "redis", Status: "disabled", Detail: "not configured"}
	}
	if !l.svcCtx.Redis.Ping() {
		return types.HealthItem{Key: "redis", Status: "error", Detail: "redis unreachable"}
	}
	return types.HealthItem{Key: "redis", Status: "ok", Detail: "redis reachable"}
}

func (l *SystemHealthLogic) databaseItem() types.HealthItem {
	ctx, cancel := context.WithTimeout(l.ctx, 2*time.Second)
	defer cancel()
	var one int
	if err := l.svcCtx.DB.QueryRowCtx(ctx, &one, "SELECT 1"); err != nil {
		return types.HealthItem{Key: "database", Status: "error", Detail: "database unreachable"}
	}
	return types.HealthItem{Key: "database", Status: "ok", Detail: "database reachable"}
}

func (l *SystemHealthLogic) coreControlItem() types.HealthItem {
	if l.svcCtx.CoreControl == nil {
		return types.HealthItem{Key: "core-control", Status: "disabled", Detail: "not configured"}
	}
	ctx, cancel := context.WithTimeout(l.ctx, 2*time.Second)
	defer cancel()
	if _, err := l.svcCtx.CoreControl.Health(ctx); err != nil {
		return types.HealthItem{Key: "core-control", Status: "degraded", Detail: "unreachable"}
	}
	return types.HealthItem{Key: "core-control", Status: "ok", Detail: "reachable"}
}

func (l *SystemHealthLogic) agentItem() types.HealthItem {
	if l.svcCtx.Agent == nil {
		return types.HealthItem{Key: "agent", Status: "disabled", Detail: "not configured"}
	}
	ctx, cancel := context.WithTimeout(l.ctx, 2*time.Second)
	defer cancel()
	if _, err := l.svcCtx.Agent.Health(ctx, &agentv1.AgentHealthRequest{}); err != nil {
		return types.HealthItem{Key: "agent", Status: "degraded", Detail: "unreachable"}
	}
	return types.HealthItem{Key: "agent", Status: "ok", Detail: "reachable"}
}

func (l *SystemHealthLogic) registrationItem() types.HealthItem {
	if !l.svcCtx.Config.Registration.Enabled {
		return types.HealthItem{Key: "registration", Status: "disabled", Detail: "RegistrationService disabled"}
	}
	return types.HealthItem{Key: "registration", Status: "ok", Detail: "enabled"}
}
