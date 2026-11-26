// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package dashboard

import (
	"net/http"

	"github.com/relayx-tech/panel/internal/logic/dashboard"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 控制台指标卡与横幅
func DashboardSummaryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := dashboard.NewDashboardSummaryLogic(r.Context(), svcCtx)
		resp, err := l.DashboardSummary()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
