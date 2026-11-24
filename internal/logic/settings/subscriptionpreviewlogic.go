// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type SubscriptionPreviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 订阅 URL 预览
func NewSubscriptionPreviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubscriptionPreviewLogic {
	return &SubscriptionPreviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubscriptionPreviewLogic) SubscriptionPreview(req *types.SubscriptionPreviewReq) (resp *types.SubscriptionPreviewResp, err error) {
	groupID := strings.TrimSpace(req.EntryGroupId)
	if groupID == "" {
		return nil, response.MissingField("entryGroupId")
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		return nil, response.MissingField("format")
	}
	formats, err := l.svcCtx.SubscriptionModel.ListFormats(l.ctx)
	if err != nil {
		return nil, response.Internal(err)
	}
	supported := false
	for _, f := range formats {
		if f.Key == format && f.Enabled && f.CapabilitySupported {
			supported = true
			break
		}
	}
	if !supported {
		return nil, response.NewFieldError(errcode.VALIDATION_UNSUPPORTED_FORMAT, "format", "unsupported subscription format")
	}

	base := strings.TrimRight(l.svcCtx.Config.PublicURL, "/")
	path := fmt.Sprintf("/sub/%s?format=%s", url.PathEscape(groupID), url.QueryEscape(format))
	return &types.SubscriptionPreviewResp{
		Code:      errcode.OK,
		Data:      types.SubscriptionPreviewData{Url: base + path, Format: format},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
