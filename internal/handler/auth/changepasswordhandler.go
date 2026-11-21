// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package auth

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/zeromicro/go-zero/rest/httpx"

	panelauth "github.com/relayx-tech/panel/internal/auth"
	logic "github.com/relayx-tech/panel/internal/logic/auth"
	"github.com/relayx-tech/panel/internal/middleware"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

// 修改当前用户密码（成功后吊销全部会话）
func ChangePasswordHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ChangePasswordReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, response.InvalidRequest(err))
			return
		}

		session := panelauth.SessionFromContext(r.Context())
		if session == nil {
			httpx.ErrorCtx(r.Context(), w, response.Unauthorized())
			return
		}

		l := logic.NewChangePasswordLogic(r.Context(), svcCtx)
		resp, err := l.ChangePassword(&req, session.User.ID,
			middleware.RemoteIP(r, svcCtx.Config.Auth.TrustProxyHeaders, svcCtx.Config.Auth.TrustedProxyCIDRs))
		if err != nil {
			var appErr *response.AppError
			if errors.As(err, &appErr) && appErr.RetryAfterSeconds > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(appErr.RetryAfterSeconds))
			}
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		response.ClearSessionCookie(w, svcCtx.CookieConfig())
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
