package integration

import (
	"context"
	"net"
	"testing"
	"time"

	corev1 "github.com/relayx-tech/contracts/gen/go/core/v1"
	"github.com/relayx-tech/panel/internal/client/corecontrol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// AC17-02：core-control 不可用时 Panel 返回错误，不本地降级拼 JSON。
func TestCoreControlClientUnavailable(t *testing.T) {
	// 取一个无人监听的端口，模拟 core-control 宕机。
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	_ = lis.Close()

	client, err := corecontrol.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.Generate(ctx, &corev1.GenerateRequest{
		Meta: &corev1.RequestMeta{OperationId: "op-down"},
		Spec: &corev1.NodeConfigSpec{NodeId: "node-1"},
	})
	if err == nil || resp != nil {
		t.Fatalf("core-control 不可用时应返回错误且无降级结果: resp=%+v err=%v", resp, err)
	}
	code := status.Code(err)
	if code != codes.Unavailable && code != codes.DeadlineExceeded {
		t.Fatalf("错误码 = %s，期望 Unavailable/DeadlineExceeded", code)
	}
}
