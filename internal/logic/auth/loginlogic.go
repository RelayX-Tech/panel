// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package auth

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/observability"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

const (
	maxAccountLength  = 64
	maxPasswordLength = 128
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 账号密码登录
func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Login 校验凭证并建立会话；成功后返回响应体与会话明文 token（仅用于写 Cookie）。
func (l *LoginLogic) Login(req *types.LoginReq, clientIP, userAgent string) (*types.LoginResp, string, error) {
	account := strings.TrimSpace(req.Account)
	if account == "" {
		return nil, "", response.MissingField("account")
	}
	if utf8.RuneCountInString(account) > maxAccountLength {
		return nil, "", response.InvalidField("account", "account must be at most 64 characters")
	}
	if req.Password == "" {
		return nil, "", response.MissingField("password")
	}
	if utf8.RuneCountInString(req.Password) > maxPasswordLength {
		return nil, "", response.InvalidField("password", "password must be at most 128 characters")
	}

	user, err := l.svcCtx.UserModel.FindByAccount(l.ctx, account)
	if errors.Is(err, model.ErrNotFound) {
		// 等成本假校验，避免通过响应时间枚举账号
		_, _ = auth.VerifyDummy(req.Password)
		l.logFailure(account, clientIP, "invalid_credentials")
		return nil, "", response.InvalidCredentials()
	}
	if err != nil {
		l.Errorw("auth.login.failure", logx.Field("requestId", requestid.FromContext(l.ctx)),
			logx.Field("userHash", observability.UserHash(account)), logx.Field("clientIp", clientIP), logx.Field("reason", "internal"))
		return nil, "", response.Internal(err)
	}
	if user.DisabledAt != nil {
		_, _ = auth.VerifyDummy(req.Password)
		l.logFailure(account, clientIP, "invalid_credentials")
		return nil, "", response.InvalidCredentials()
	}

	ok, err := auth.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil {
		return nil, "", response.Internal(err)
	}
	if !ok {
		l.logFailure(account, clientIP, "invalid_credentials")
		return nil, "", response.InvalidCredentials()
	}

	token, sess, err := l.svcCtx.SessionManager.Create(l.ctx, user.ID, clientIP, truncate(userAgent, 512))
	if err != nil {
		l.Errorw("auth.login.failure", logx.Field("requestId", requestid.FromContext(l.ctx)),
			logx.Field("userHash", observability.UserHash(account)), logx.Field("reason", "session_create"))
		return nil, "", response.Internal(err)
	}

	l.Infow("auth.login.success", logx.Field("requestId", requestid.FromContext(l.ctx)),
		logx.Field("userHash", observability.UserHash(user.Account)), logx.Field("userId", user.ID),
		logx.Field("clientIp", clientIP), logx.Field("result", "success"))

	return &types.LoginResp{
		Code:    "OK",
		Message: "",
		Data: types.LoginData{
			User: types.UserInfo{
				Id:          user.ID,
				Account:     user.Account,
				DisplayName: user.DisplayName,
			},
			Session: types.SessionInfo{
				ExpiresAt:         formatTime(sess.ExpiresAt),
				AbsoluteExpiresAt: formatTime(sess.AbsoluteExpiresAt),
			},
		},
		RequestId: requestid.FromContext(l.ctx),
	}, token, nil
}

func (l *LoginLogic) logFailure(account, clientIP, reason string) {
	l.Infow("auth.login.failure", logx.Field("requestId", requestid.FromContext(l.ctx)),
		logx.Field("userHash", observability.UserHash(account)), logx.Field("clientIp", clientIP),
		logx.Field("result", "failure"), logx.Field("reason", reason))
}

// formatTime 输出 RFC3339 毫秒 UTC（design.md §3.2）。
func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
