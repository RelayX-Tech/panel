// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package upgrade

import (
	"net/http"

	"github.com/relayx-tech/panel/internal/logic/upgrade"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 任务详情
func UpgradeTaskDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UpgradeTaskDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := upgrade.NewUpgradeTaskDetailLogic(r.Context(), svcCtx)
		resp, err := l.UpgradeTaskDetail(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
