// Package grpcerror 把 gRPC status + ErrorDetail 转换为 Panel HTTP 错误（design §6.8）。
package grpcerror

import (
	"net/http"

	agentv1 "github.com/relayx-tech/contracts/gen/go/agent/v1"
	corev1 "github.com/relayx-tech/contracts/gen/go/core/v1"
	"github.com/relayx-tech/panel/internal/errcode"
	"google.golang.org/grpc/status"
)

// Detail 是字段级错误项。
type Detail struct {
	Field   string
	Code    string
	Message string
}

// HTTPError 是转换后的 HTTP 错误。
type HTTPError struct {
	Code      string
	HTTP      int
	Field     string
	RequestID string
	Retryable bool
	Message   string
	Details   []Detail
}

// Convert 把 gRPC 错误转换为 HTTP 错误；未知码兜底 INTERNAL / 500。
func Convert(err error) *HTTPError {
	if err == nil {
		return nil
	}
	st, _ := status.FromError(err)
	out := &HTTPError{Code: errcode.INTERNAL, HTTP: http.StatusInternalServerError, Message: st.Message()}
	for _, d := range st.Details() {
		switch detail := d.(type) {
		case *corev1.ErrorDetail:
			apply(out, detail.GetCode(), detail.GetField(), detail.GetRequestId(), detail.GetRetryable(), detail.GetMessage(), coreItems(detail))
		case *agentv1.AgentErrorDetail:
			apply(out, detail.GetCode(), detail.GetField(), detail.GetRequestId(), detail.GetRetryable(), detail.GetMessage(), agentItems(detail))
		}
	}
	return out
}

func apply(out *HTTPError, code, field, requestID string, retryable bool, message string, details []Detail) {
	if code != "" {
		out.Code = code
	}
	out.Field = field
	out.RequestID = requestID
	out.Retryable = retryable
	if message != "" {
		out.Message = message
	}
	out.Details = details
	if httpStatus, ok := errcode.HTTPStatus[code]; ok {
		out.HTTP = httpStatus
	} else {
		out.HTTP = http.StatusInternalServerError
	}
}

func coreItems(d *corev1.ErrorDetail) []Detail {
	out := make([]Detail, 0, len(d.GetDetails()))
	for _, it := range d.GetDetails() {
		out = append(out, Detail{Field: it.GetField(), Code: it.GetCode(), Message: it.GetMessage()})
	}
	return out
}

func agentItems(d *agentv1.AgentErrorDetail) []Detail {
	out := make([]Detail, 0, len(d.GetDetails()))
	for _, it := range d.GetDetails() {
		out = append(out, Detail{Field: it.GetField(), Code: it.GetCode(), Message: it.GetMessage()})
	}
	return out
}
