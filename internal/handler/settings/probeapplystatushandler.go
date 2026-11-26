// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"net/http"

	"github.com/relayx-tech/panel/internal/logic/settings"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 探测规格下发进度
func ProbeApplyStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := settings.NewProbeApplyStatusLogic(r.Context(), svcCtx)
		resp, err := l.ProbeApplyStatus()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
