// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package alerts

import (
	"net/http"

	"github.com/relayx-tech/panel/internal/logic/alerts"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 告警详情
func AlertDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AlertDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := alerts.NewAlertDetailLogic(r.Context(), svcCtx)
		resp, err := l.AlertDetail(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
