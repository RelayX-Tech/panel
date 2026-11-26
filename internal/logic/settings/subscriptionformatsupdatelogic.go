// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type SubscriptionFormatsUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 保存订阅格式开关
func NewSubscriptionFormatsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubscriptionFormatsUpdateLogic {
	return &SubscriptionFormatsUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubscriptionFormatsUpdateLogic) SubscriptionFormatsUpdate(req *types.SubscriptionFormatsUpdateReq) (resp *types.SubscriptionFormatsResp, err error) {
	for _, f := range req.Formats {
		if strings.TrimSpace(f.Key) == "" {
			return nil, response.MissingField("formats.key")
		}
		if err := l.svcCtx.SubscriptionModel.SetFormatEnabled(l.ctx, f.Key, f.Enabled, f.SortOrder); err != nil {
			return nil, response.Internal(err)
		}
	}
	formats, err := l.svcCtx.SubscriptionModel.ListFormats(l.ctx)
	if err != nil {
		return nil, response.Internal(err)
	}
	out := mergeFormats(formats, l.svcCtx.Config.Subscription.Capabilities)
	return &types.SubscriptionFormatsResp{
		Code:      errcode.OK,
		Data:      types.SubscriptionFormatsData{Formats: out},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
