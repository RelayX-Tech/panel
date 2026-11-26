// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"net/http"

	"github.com/relayx-tech/panel/internal/logic/settings"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 安全策略（只读）
func SecuritySettingsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := settings.NewSecuritySettingsLogic(r.Context(), svcCtx)
		resp, err := l.SecuritySettings()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
