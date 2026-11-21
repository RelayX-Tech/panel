// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package auth

import (
	"context"
	"errors"
	"unicode"
	"unicode/utf8"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

const (
	minNewPasswordLength = 12
	maxNewPasswordLength = 128
	maxOldPasswordLength = 128
)

// 事务内控制流错误：用于回滚并区分「旧密码错误」与「新旧相同」，
// 不直接返回给客户端（返回前转换为稳定的 AppError）。
var (
	errOldPasswordIncorrect = errors.New("old password incorrect")
	errNewPasswordSameAsOld = errors.New("new password same as old")
)

type ChangePasswordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 修改当前用户密码（成功后吊销全部会话）
func NewChangePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangePasswordLogic {
	return &ChangePasswordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ChangePassword 校验旧密码并写入新哈希；成功后吊销该用户全部会话并写审计（design.md §3.6 / §5.1）。
func (l *ChangePasswordLogic) ChangePassword(req *types.ChangePasswordReq, userID, clientIP string) (resp *types.ChangePasswordResp, err error) {
	if appErr := validateRequest(req); appErr != nil {
		return nil, appErr
	}

	if l.svcCtx.PasswordLimiter == nil {
		return nil, response.Internal(errors.New("password limiter is not configured"))
	}

	// 锁定期间直接 429：不查库、不校验旧密码（AC23）。
	locked, retryAfter, err := l.svcCtx.PasswordLimiter.Check(l.ctx, userID)
	if err != nil {
		return nil, response.Internal(err)
	}
	if locked {
		return nil, response.RateLimited(retryAfter)
	}

	// 新哈希在事务外预计算，缩短行锁持有时间（design.md §3.6）。
	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return nil, response.Internal(err)
	}

	err = l.svcCtx.Tx.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		user, err := l.svcCtx.UserModel.LockByID(ctx, session, userID)
		if errors.Is(err, model.ErrNotFound) {
			return response.SessionExpired()
		}
		if err != nil {
			return response.Internal(err)
		}

		ok, err := auth.VerifyPassword(req.OldPassword, user.PasswordHash)
		if err != nil {
			return response.Internal(err)
		}
		if !ok {
			return errOldPasswordIncorrect
		}
		if req.NewPassword == req.OldPassword {
			return errNewPasswordSameAsOld
		}

		if err := l.svcCtx.UserModel.UpdatePasswordHash(ctx, session, userID, newHash); err != nil {
			return response.Internal(err)
		}
		if _, err := l.svcCtx.SessionModel.RevokeAllByUserID(ctx, session, userID); err != nil {
			return response.Internal(err)
		}
		// 审计与改密同事务：写失败整体回滚（fail-closed）。
		if err := l.svcCtx.AuditModel.Insert(ctx, session, &model.PasswordAudit{
			UserID:    userID,
			RequestID: requestid.FromContext(l.ctx),
			ClientIP:  clientIP,
			Result:    "success",
		}); err != nil {
			return response.Internal(err)
		}
		return nil
	})

	switch {
	case errors.Is(err, errOldPasswordIncorrect):
		return nil, l.recordFailure(userID, clientIP)
	case errors.Is(err, errNewPasswordSameAsOld):
		return nil, response.NewPasswordSameAsOld()
	case err != nil:
		return nil, err
	}

	// 提交后的 best-effort 副作用：失败只告警，不改变已提交结果。
	if err := l.svcCtx.UserModel.InvalidateCache(l.ctx, userID); err != nil {
		l.Errorw("auth.password.change", logx.Field("requestId", requestid.FromContext(l.ctx)),
			logx.Field("userId", userID), logx.Field("result", "cache_invalidate_failed"),
			logx.Field("error", err.Error()))
	}
	if err := l.svcCtx.PasswordLimiter.Reset(l.ctx, userID); err != nil {
		l.Errorw("auth.password.change", logx.Field("requestId", requestid.FromContext(l.ctx)),
			logx.Field("userId", userID), logx.Field("result", "limiter_reset_failed"),
			logx.Field("error", err.Error()))
	}

	l.Infow("auth.password.change", logx.Field("requestId", requestid.FromContext(l.ctx)),
		logx.Field("userId", userID), logx.Field("clientIp", clientIP), logx.Field("result", "success"))

	return &types.ChangePasswordResp{
		Code:      errcode.OK,
		Message:   "",
		Data:      nil,
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}

func (l *ChangePasswordLogic) recordFailure(userID, clientIP string) error {
	count, retryAfter, err := l.svcCtx.PasswordLimiter.RecordFailure(l.ctx, userID)
	if err != nil {
		return response.Internal(err)
	}
	l.Infow("auth.password.change", logx.Field("requestId", requestid.FromContext(l.ctx)),
		logx.Field("userId", userID), logx.Field("clientIp", clientIP),
		logx.Field("result", "failure"), logx.Field("reason", "invalid_old_password"))
	if count >= l.svcCtx.Config.Auth.PasswordChangeRateLimit.MaxAttempts {
		return response.RateLimited(retryAfter)
	}
	return response.InvalidOldPassword()
}

// validateRequest 做字段级校验（不 trim、按 Unicode code point 计数）；失败不发请求、不计数。
func validateRequest(req *types.ChangePasswordReq) *response.AppError {
	if req.OldPassword == "" {
		return response.MissingField("oldPassword")
	}
	if req.NewPassword == "" {
		return response.MissingField("newPassword")
	}
	if utf8.RuneCountInString(req.OldPassword) > maxOldPasswordLength {
		return response.InvalidField("oldPassword", "old password must be at most 128 characters")
	}

	length := utf8.RuneCountInString(req.NewPassword)
	switch {
	case length < minNewPasswordLength:
		return response.InvalidNewPassword("TOO_SHORT", "new password must be at least 12 characters")
	case length > maxNewPasswordLength:
		return response.InvalidNewPassword("TOO_LONG", "new password must be at most 128 characters")
	case !hasNonWhitespace(req.NewPassword):
		return response.InvalidNewPassword("NO_PRINTABLE", "new password must contain at least one non-whitespace character")
	}
	return nil
}

func hasNonWhitespace(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}
