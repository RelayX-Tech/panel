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

// 隧道链路
func MonitoringTunnelsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TunnelListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := monitoring.NewMonitoringTunnelsLogic(r.Context(), svcCtx)
		resp, err := l.MonitoringTunnels(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
