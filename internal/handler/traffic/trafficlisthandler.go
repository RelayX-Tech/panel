// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package traffic

import (
	"net/http"

	"github.com/relayx-tech/panel/internal/logic/traffic"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 流量分组表
func TrafficListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TrafficListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := traffic.NewTrafficListLogic(r.Context(), svcCtx)
		resp, err := l.TrafficList(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
