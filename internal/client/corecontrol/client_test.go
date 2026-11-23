package corecontrol_test

import (
	"context"
	"net"
	"testing"

	corev1 "github.com/relayx-tech/contracts/gen/go/core/v1"
	"github.com/relayx-tech/panel/internal/client/corecontrol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type fakeConfig struct {
	corev1.UnimplementedConfigServiceServer
}

func (f *fakeConfig) Generate(context.Context, *corev1.GenerateRequest) (*corev1.GenerateResponse, error) {
	return &corev1.GenerateResponse{XrayConfigJson: `{}`, ConfigHash: "hash", XrayVersion: "26.11.9"}, nil
}

func dial(t *testing.T, srv *grpc.Server, lis *bufconn.Listener) *grpc.ClientConn {
	t.Helper()
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		srv.Stop()
	})
	return conn
}

func TestGenerate(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	corev1.RegisterConfigServiceServer(srv, &fakeConfig{})
	client := corecontrol.New(dial(t, srv, lis))

	resp, err := client.Generate(context.Background(), &corev1.GenerateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetConfigHash() != "hash" || resp.GetXrayVersion() != "26.11.9" {
		t.Fatalf("resp = %+v", resp)
	}
}
