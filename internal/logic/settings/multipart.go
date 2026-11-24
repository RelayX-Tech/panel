package settings

import (
	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/response"
)

// InvalidMultipart 把 multipart 解析失败映射为字段级 400。
func InvalidMultipart(err error) *response.AppError {
	return response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "file", "invalid multipart upload: "+err.Error())
}
