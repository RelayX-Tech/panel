// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package upgrade

import (
	"net/http"

	"github.com/relayx-tech/panel/internal/logic/upgrade"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 升级状态与锁
func UpgradeStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := upgrade.NewUpgradeStatusLogic(r.Context(), svcCtx)
		resp, err := l.UpgradeStatus()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
