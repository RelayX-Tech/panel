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

// 发起 Panel 升级
func UpgradePanelHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UpgradePanelReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := upgrade.NewUpgradePanelLogic(r.Context(), svcCtx)
		resp, err := l.UpgradePanel(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
