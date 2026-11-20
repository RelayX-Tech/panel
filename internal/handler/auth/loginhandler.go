// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package auth

import (
	"net/http"

	logic "github.com/relayx-tech/panel/internal/logic/auth"
	"github.com/relayx-tech/panel/internal/middleware"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 账号密码登录
func LoginHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LoginReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewLoginLogic(r.Context(), svcCtx)
		resp, token, err := l.Login(&req,
			middleware.RemoteIP(r, svcCtx.Config.Auth.TrustProxyHeaders, svcCtx.Config.Auth.TrustedProxyCIDRs),
			r.UserAgent())
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		response.SetSessionCookie(w, svcCtx.CookieConfig(), token)
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
