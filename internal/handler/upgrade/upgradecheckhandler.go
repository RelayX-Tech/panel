// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package upgrade

import (
	"net/http"

	"github.com/relayx-tech/panel/internal/logic/upgrade"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 手动检查更新
func UpgradeCheckHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := upgrade.NewUpgradeCheckLogic(r.Context(), svcCtx)
		resp, err := l.UpgradeCheck()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
