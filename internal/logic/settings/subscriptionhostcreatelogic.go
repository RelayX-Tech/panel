// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type SubscriptionHostCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 新增订阅域名
func NewSubscriptionHostCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubscriptionHostCreateLogic {
	return &SubscriptionHostCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubscriptionHostCreateLogic) SubscriptionHostCreate(req *types.SubscriptionHostCreateReq) (resp *types.SubscriptionHostItemResp, err error) {
	host := strings.TrimSpace(strings.ToLower(req.Host))
	if host == "" {
		return nil, response.MissingField("host")
	}
	if !validHost(host) {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "host", "invalid domain")
	}
	if strings.TrimSpace(req.EntryGroupId) == "" {
		return nil, response.MissingField("entryGroupId")
	}
	tlsMode := strings.ToLower(strings.TrimSpace(req.TlsMode))
	if tlsMode == "" {
		tlsMode = "panel"
	}
	if !validTLSMode(tlsMode) {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "tlsMode", "unsupported tlsMode")
	}

	created, err := l.svcCtx.SubscriptionModel.CreateHost(l.ctx, model.SubscriptionHost{
		Host: host, EntryGroupID: req.EntryGroupId, TLSMode: tlsMode,
	})
	if err != nil {
		if sameHostExists(err) {
			return nil, response.NewError(errcode.CONFLICT_SUBSCRIPTION_HOST_EXISTS, "subscription host already exists")
		}
		return nil, response.Internal(err)
	}
	if l.svcCtx.AuditLogModel != nil {
		actor := ""
		if row := auth.SessionFromContext(l.ctx); row != nil {
			actor = row.User.Account
		}
		_ = l.svcCtx.AuditLogModel.Insert(l.ctx, model.AuditLog{
			Actor: actor, Action: "subscription.host.create", ObjectType: "subscription_host",
			ObjectID: host, Result: "success", RequestID: requestid.FromContext(l.ctx), CreatedAt: time.Now().UTC(),
		})
	}
	return &types.SubscriptionHostItemResp{
		Code:      errcode.OK,
		Data:      types.SubscriptionHostItemData{Item: toSubscriptionHost(*created)},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
