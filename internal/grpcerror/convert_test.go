package grpcerror_test

import (
	"errors"
	"testing"

	agentv1 "github.com/relayx-tech/contracts/gen/go/agent/v1"
	corev1 "github.com/relayx-tech/contracts/gen/go/core/v1"
	"github.com/relayx-tech/panel/internal/grpcerror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func grpcErrWithCoreDetail(t *testing.T, detail *corev1.ErrorDetail) error {
	t.Helper()
	st, err := status.New(codes.InvalidArgument, detail.GetMessage()).WithDetails(detail)
	if err != nil {
		t.Fatal(err)
	}
	return st.Err()
}

func TestConvertCoreError(t *testing.T) {
	err := grpcErrWithCoreDetail(t, &corev1.ErrorDetail{
		Code:      "VALIDATION_INVALID_PORT",
		Field:     "port_groups[0].ports[0].port",
		Message:   "端口不合法",
		RequestId: "req-1",
		Details:   []*corev1.ErrorItem{{Field: "port_groups[0].ports[0].port", Code: "OUT_OF_RANGE", Message: "端口须在 2000-30000"}},
	})
	out := grpcerror.Convert(err)
	if out == nil {
		t.Fatal("转换结果为空")
	}
	if out.Code != "VALIDATION_INVALID_PORT" || out.HTTP != 400 {
		t.Fatalf("out = %+v", out)
	}
	if out.Field == "" || out.RequestID != "req-1" || len(out.Details) != 1 {
		t.Fatalf("out = %+v", out)
	}
}

func TestConvertAgentError(t *testing.T) {
	st, err := status.New(codes.FailedPrecondition, "不兼容").WithDetails(&agentv1.AgentErrorDetail{
		Code: "VERSION_INCOMPATIBLE", Message: "合同版本不兼容",
	})
	if err != nil {
		t.Fatal(err)
	}
	out := grpcerror.Convert(st.Err())
	if out.Code != "VERSION_INCOMPATIBLE" || out.HTTP != 426 {
		t.Fatalf("out = %+v", out)
	}
}

func TestConvertPlainError(t *testing.T) {
	out := grpcerror.Convert(errors.New("boom"))
	if out.Code != "INTERNAL" || out.HTTP != 500 {
		t.Fatalf("out = %+v", out)
	}
}
