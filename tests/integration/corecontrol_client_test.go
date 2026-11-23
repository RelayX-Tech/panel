package integration

import (
	"context"
	"net"
	"testing"
	"time"

	corev1 "github.com/relayx-tech/contracts/gen/go/core/v1"
	"github.com/relayx-tech/panel/internal/client/corecontrol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// fakeCoreControl 是 ConfigService 的真实 gRPC 服务端（模拟 core-control）。
type fakeCoreControl struct {
	corev1.UnimplementedConfigServiceServer
	json string
}

func (f *fakeCoreControl) Generate(context.Context, *corev1.GenerateRequest) (*corev1.GenerateResponse, error) {
	return &corev1.GenerateResponse{
		XrayConfigJson: f.json, ConfigHash: "hash-core", SchemaVersion: "1.0.0", XrayVersion: "26.11.9",
	}, nil
}

func (f *fakeCoreControl) Validate(context.Context, *corev1.ValidateRequest) (*corev1.ValidateResponse, error) {
	return &corev1.ValidateResponse{Valid: true, SchemaVersion: "1.0.0"}, nil
}

func startCoreControl(t *testing.T, f *fakeCoreControl) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	corev1.RegisterConfigServiceServer(srv, f)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

// AC17-01：Panel 经真实 gRPC 调 core-control 生成 / 校验；配置 JSON 原样来自 core-control，Panel 不拼。
func TestCoreControlClientGenerateAndValidate(t *testing.T) {
	const wantJSON = `{"inbounds":[],"outbounds":[]}`
	addr := startCoreControl(t, &fakeCoreControl{json: wantJSON})

	client, err := corecontrol.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	gen, err := client.Generate(ctx, &corev1.GenerateRequest{
		Meta: &corev1.RequestMeta{OperationId: "op-gen"},
		Spec: &corev1.NodeConfigSpec{NodeId: "node-1"},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if gen.GetXrayConfigJson() != wantJSON || gen.GetConfigHash() != "hash-core" {
		t.Fatalf("Panel 不应改写 core-control 输出: %+v", gen)
	}

	val, err := client.Validate(ctx, &corev1.ValidateRequest{
		Meta: &corev1.RequestMeta{OperationId: "op-val"},
		Spec: &corev1.NodeConfigSpec{NodeId: "node-1"},
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !val.GetValid() {
		t.Fatalf("validate = %+v", val)
	}
}
