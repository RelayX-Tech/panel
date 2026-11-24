// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type SubscriptionHostUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 编辑订阅域名
func NewSubscriptionHostUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubscriptionHostUpdateLogic {
	return &SubscriptionHostUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubscriptionHostUpdateLogic) SubscriptionHostUpdate(req *types.SubscriptionHostUpdateReq) (resp *types.SubscriptionHostItemResp, err error) {
	id, perr := strconv.ParseInt(req.Id, 10, 64)
	if perr != nil || id <= 0 {
		return nil, response.NewError(errcode.NOT_FOUND_SUBSCRIPTION_HOST, "subscription host not found")
	}
	if req.EntryGroupId != nil && strings.TrimSpace(*req.EntryGroupId) == "" {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "entryGroupId", "entryGroupId is required")
	}
	if req.TlsMode != nil && !validTLSMode(*req.TlsMode) {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "tlsMode", "unsupported tlsMode")
	}
	var tlsMode *string
	if req.TlsMode != nil {
		mode := strings.ToLower(*req.TlsMode)
		tlsMode = &mode
	}
	updated, err := l.svcCtx.SubscriptionModel.UpdateHost(l.ctx, id, req.EntryGroupId, tlsMode)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, response.NewError(errcode.NOT_FOUND_SUBSCRIPTION_HOST, "subscription host not found")
		}
		return nil, response.Internal(err)
	}
	return &types.SubscriptionHostItemResp{
		Code:      errcode.OK,
		Data:      types.SubscriptionHostItemData{Item: toSubscriptionHost(*updated)},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
