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
	return &AppError{Status: errcode.HTTPStatus[errcode.RATE_LIMITED], Code: errcode.RATE_LIMITED, Message: "too many attempts"}
}

func Internal(err error) *AppError {
	return &AppError{Status: errcode.HTTPStatus[errcode.INTERNAL], Code: errcode.INTERNAL, Message: "internal error", Err: err}
}
