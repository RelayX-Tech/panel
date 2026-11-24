// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package nodes

import (
	"net/http"

	"github.com/relayx-tech/panel/internal/logic/nodes"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 立即拨测
func NodeProbeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.NodeProbeReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := nodes.NewNodeProbeLogic(r.Context(), svcCtx)
		resp, err := l.NodeProbe(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
