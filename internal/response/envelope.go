package response

import (
	"context"
	"errors"
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/requestid"
)

// Envelope 是成功响应包装（design.md §3.2）。
type Envelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestId string `json:"requestId"`
}

// ErrorEnvelope 是失败响应包装；field/details 仅在字段级错误时出现。
type ErrorEnvelope struct {
	Code      string       `json:"code"`
	Message   string       `json:"message"`
	Field     string       `json:"field,omitempty"`
	Details   []FieldError `json:"details,omitempty"`
	Data      any          `json:"data"`
	RequestId string       `json:"requestId"`
}

// FieldError 描述单个字段的校验错误。
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// AppError 是传输层可识别的业务错误：稳定错误码 + HTTP 状态。
type AppError struct {
	Status  int
	Code    string
	Message string
	Field   string
	Details []FieldError
	// RetryAfterSeconds 非零时由 handler 写入 Retry-After 响应头（429 限流）。
	RetryAfterSeconds int
	// Err 仅用于服务端日志，不返回给客户端。
	Err error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

// OK 输出统一成功包装。
func OK(w http.ResponseWriter, r *http.Request, data any) {
	httpx.OkJsonCtx(r.Context(), w, Envelope{
		Code:      errcode.OK,
		Message:   "",
		Data:      data,
		RequestId: requestid.FromContext(r.Context()),
	})
}

// InstallErrorHandler 注册全局错误处理器：AppError 按稳定错误码输出，
// 其他错误统一为 500 INTERNAL，不泄露内部细节。
func InstallErrorHandler() {
	httpx.SetErrorHandlerCtx(func(ctx context.Context, err error) (int, any) {
		var appErr *AppError
		if errors.As(err, &appErr) {
			status := appErr.Status
			if status == 0 {
				status = http.StatusInternalServerError
			}
			code := appErr.Code
			if code == "" {
				code = errcode.INTERNAL
			}
			return status, ErrorEnvelope{
				Code:      code,
				Message:   appErr.Message,
				Field:     appErr.Field,
				Details:   appErr.Details,
				Data:      nil,
				RequestId: requestid.FromContext(ctx),
			}
		}
		return http.StatusInternalServerError, ErrorEnvelope{
			Code:      errcode.INTERNAL,
			Message:   "internal error",
			Data:      nil,
			RequestId: requestid.FromContext(ctx),
		}
	})
}

// 常用错误构造器（错误码目录见 design.md §3.4）。

func InvalidCredentials() *AppError {
	return &AppError{Status: errcode.HTTPStatus[errcode.AUTH_INVALID_CREDENTIALS], Code: errcode.AUTH_INVALID_CREDENTIALS, Message: "invalid credentials"}
}

func Unauthorized() *AppError {
	return &AppError{Status: errcode.HTTPStatus[errcode.AUTH_UNAUTHORIZED], Code: errcode.AUTH_UNAUTHORIZED, Message: "authentication required"}
}

func SessionExpired() *AppError {
	return &AppError{Status: errcode.HTTPStatus[errcode.AUTH_SESSION_EXPIRED], Code: errcode.AUTH_SESSION_EXPIRED, Message: "session expired"}
}

func CSRFRejected() *AppError {
	return &AppError{Status: errcode.HTTPStatus[errcode.AUTH_CSRF_REJECTED], Code: errcode.AUTH_CSRF_REJECTED, Message: "origin not allowed"}
}

func MissingField(field string) *AppError {
	return &AppError{
		Status:  errcode.HTTPStatus[errcode.VALIDATION_MISSING_FIELD],
		Code:    errcode.VALIDATION_MISSING_FIELD,
		Message: "required field missing",
		Field:   field,
		Details: []FieldError{{Field: field, Code: "REQUIRED", Message: field + " is required"}},
	}
}

func InvalidField(field, message string) *AppError {
	return &AppError{
		Status:  errcode.HTTPStatus[errcode.VALIDATION_INVALID_FIELD],
		Code:    errcode.VALIDATION_INVALID_FIELD,
		Message: "invalid field",
		Field:   field,
		Details: []FieldError{{Field: field, Code: "INVALID", Message: message}},
	}
}

// InvalidRequest 把请求体解析失败映射为 400 VALIDATION_INVALID_FIELD，
// 避免畸形 / 空 body 穿透成 500 INTERNAL（design.md §3.6）。
func InvalidRequest(err error) *AppError {
	return &AppError{
		Status:  errcode.HTTPStatus[errcode.VALIDATION_INVALID_FIELD],
		Code:    errcode.VALIDATION_INVALID_FIELD,
		Message: "invalid request body",
		Field:   "body",
		Details: []FieldError{{Field: "body", Code: "INVALID", Message: "request body is not valid JSON"}},
		Err:     err,
	}
}

func RateLimited(retryAfterSeconds int) *AppError {
	return &AppError{
		Status:            errcode.HTTPStatus[errcode.RATE_LIMITED],
		Code:              errcode.RATE_LIMITED,
		Message:           "too many attempts",
		RetryAfterSeconds: retryAfterSeconds,
	}
}

// InvalidOldPassword 表示修改密码时旧密码校验失败（400，字段级，不触发全局 401）。
func InvalidOldPassword() *AppError {
	const message = "old password is incorrect"
	return &AppError{
		Status:  errcode.HTTPStatus[errcode.VALIDATION_INVALID_OLD_PASSWORD],
		Code:    errcode.VALIDATION_INVALID_OLD_PASSWORD,
		Message: message,
		Field:   "oldPassword",
		Details: []FieldError{{Field: "oldPassword", Code: "INVALID_OLD_PASSWORD", Message: message}},
	}
}

// InvalidNewPassword 表示新密码不满足长度 / 非空白要求；detailCode 为
// TOO_SHORT / TOO_LONG / NO_PRINTABLE，供前端选择精确文案。
func InvalidNewPassword(detailCode, message string) *AppError {
	return &AppError{
		Status:  errcode.HTTPStatus[errcode.VALIDATION_INVALID_NEW_PASSWORD],
		Code:    errcode.VALIDATION_INVALID_NEW_PASSWORD,
		Message: "new password does not meet requirements",
		Field:   "newPassword",
		Details: []FieldError{{Field: "newPassword", Code: detailCode, Message: message}},
	}
}

// NewPasswordSameAsOld 表示新旧密码完全相同（400，字段级）。
func NewPasswordSameAsOld() *AppError {
	const message = "new password must differ from the old password"
	return &AppError{
		Status:  errcode.HTTPStatus[errcode.VALIDATION_NEW_PASSWORD_SAME_AS_OLD],
		Code:    errcode.VALIDATION_NEW_PASSWORD_SAME_AS_OLD,
		Message: message,
		Field:   "newPassword",
		Details: []FieldError{{Field: "newPassword", Code: "SAME_AS_OLD", Message: message}},
	}
}

func Internal(err error) *AppError {
	return &AppError{Status: errcode.HTTPStatus[errcode.INTERNAL], Code: errcode.INTERNAL, Message: "internal error", Err: err}
}

// NewError 按错误码注册表构造 AppError；未知错误码按 500 INTERNAL 处理。
func NewError(code, message string) *AppError {
	status := errcode.HTTPStatus[code]
	if status == 0 {
		status = 500
	}
	return &AppError{Status: status, Code: code, Message: message}
}

// NewFieldError 构造字段级错误：错误码 + field + details。
func NewFieldError(code, field, message string) *AppError {
	status := errcode.HTTPStatus[code]
	if status == 0 {
		status = 400
	}
	return &AppError{
		Status:  status,
		Code:    code,
		Message: message,
		Field:   field,
		Details: []FieldError{{Field: field, Code: code, Message: message}},
	}
}
