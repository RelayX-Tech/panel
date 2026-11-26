// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package monitoring

import (
	"net/http"

	"github.com/relayx-tech/panel/internal/logic/monitoring"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 监控总览
func MonitoringOverviewHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MonitoringOverviewReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := monitoring.NewMonitoringOverviewLogic(r.Context(), svcCtx)
		resp, err := l.MonitoringOverview(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
