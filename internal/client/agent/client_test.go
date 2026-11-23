package agent_test

import (
	"context"
	"net"
	"testing"

	agentv1 "github.com/relayx-tech/contracts/gen/go/agent/v1"
	agentclient "github.com/relayx-tech/panel/internal/client/agent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type fakeAgent struct {
	agentv1.UnimplementedAgentControlServiceServer
}

func (f *fakeAgent) Hello(context.Context, *agentv1.HelloRequest) (*agentv1.HelloResponse, error) {
	return &agentv1.HelloResponse{Compatible: true, NodeId: "node-1"}, nil
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

func TestHello(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	agentv1.RegisterAgentControlServiceServer(srv, &fakeAgent{})
	client := agentclient.New(dial(t, srv, lis))

	resp, err := client.Hello(context.Background(), &agentv1.HelloRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.GetCompatible() || resp.GetNodeId() != "node-1" {
		t.Fatalf("resp = %+v", resp)
	}
}
