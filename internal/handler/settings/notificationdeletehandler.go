// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"net/http"

	"github.com/relayx-tech/panel/internal/logic/settings"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 删除通知渠道
func NotificationDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.NotificationTestReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := settings.NewNotificationDeleteLogic(r.Context(), svcCtx)
		resp, err := l.NotificationDelete(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
