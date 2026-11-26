// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type SubscriptionFormatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 订阅格式开关
func NewSubscriptionFormatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubscriptionFormatsLogic {
	return &SubscriptionFormatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubscriptionFormatsLogic) SubscriptionFormats() (resp *types.SubscriptionFormatsResp, err error) {
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

// mergeFormats 以 DB 为准并按 capabilities 校正（design.md §3.5 API-26 / AC115）：
// DB 有、能力无 → 强制禁用并标记不支持；能力有、DB 无 → 只读兜底。
func mergeFormats(db []model.SubscriptionFormat, capabilities []string) []types.SubscriptionFormat {
	out := make([]types.SubscriptionFormat, 0, len(db)+len(capabilities))
	known := make(map[string]bool, len(db))
	maxOrder := 0
	for _, f := range db {
		known[f.Key] = true
		if f.SortOrder > maxOrder {
			maxOrder = f.SortOrder
		}
	}
	if len(capabilities) == 0 {
		for _, f := range db {
			out = append(out, types.SubscriptionFormat{
				Key: f.Key, Label: f.Label, Enabled: f.Enabled, SortOrder: f.SortOrder,
				Source: "db", CapabilitySupported: true,
			})
		}
		return out
	}
	allowed := make(map[string]bool, len(capabilities))
	for _, c := range capabilities {
		allowed[c] = true
	}
	for _, f := range db {
		supported := allowed[f.Key]
		out = append(out, types.SubscriptionFormat{
			Key: f.Key, Label: f.Label, Enabled: f.Enabled && supported, SortOrder: f.SortOrder,
			Source: "db", CapabilitySupported: supported,
		})
	}
	for _, c := range capabilities {
		if known[c] {
			continue
		}
		maxOrder++
		out = append(out, types.SubscriptionFormat{
			Key: c, Label: c, Enabled: false, SortOrder: maxOrder, Source: "capability", CapabilitySupported: true,
		})
	}
	return out
}
