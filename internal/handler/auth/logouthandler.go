// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package auth

import (
	"net/http"

	logic "github.com/relayx-tech/panel/internal/logic/auth"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 注销当前会话（幂等）
func LogoutHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := response.ReadSessionCookie(r, svcCtx.Config.Auth.SessionCookieName)

		l := logic.NewLogoutLogic(r.Context(), svcCtx)
		resp, err := l.Logout(token)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		response.ClearSessionCookie(w, svcCtx.CookieConfig())
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
