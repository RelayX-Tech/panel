package registration_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	agentv1 "github.com/relayx-tech/contracts/gen/go/agent/v1"
	"github.com/relayx-tech/panel/internal/config"
	"github.com/relayx-tech/panel/internal/logic/register"
	"github.com/relayx-tech/panel/internal/pki"
	registration "github.com/relayx-tech/panel/internal/server/registration"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func newCA(t *testing.T) *pki.CA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "panel-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return pki.NewCA(cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func newCSR(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "agent"}}, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})
}

func newServer(t *testing.T) (*registration.Server, *register.Service) {
	t.Helper()
	svc := register.New(register.NewMemoryStore(), newCA(t))
	return registration.New(svc), svc
}

func registerNode(t *testing.T, srv *registration.Server, svc *register.Service, nodeID string) {
	t.Helper()
	token, err := svc.CreateToken(context.Background(), "entry")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Register(context.Background(), &agentv1.RegisterRequest{
		Token: token, NodeId: nodeID, NodeType: agentv1.AgentNodeType_AGENT_NODE_TYPE_ENTRY, CsrPem: newCSR(t),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterAndGetCA(t *testing.T) {
	srv, svc := newServer(t)
	ctx := context.Background()

	token, err := svc.CreateToken(ctx, "entry")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.Register(ctx, &agentv1.RegisterRequest{
		Token: token, NodeId: "node-1", NodeType: agentv1.AgentNodeType_AGENT_NODE_TYPE_ENTRY, CsrPem: newCSR(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetCertChainPem()) == 0 || len(resp.GetCaPem()) == 0 || resp.GetNotAfter() == nil {
		t.Fatalf("resp = %+v", resp)
	}
	if len(resp.GetPinnedCertFingerprintSha256()) != 64 {
		t.Fatalf("fingerprint = %q", resp.GetPinnedCertFingerprintSha256())
	}

	ca, err := srv.GetCA(ctx, &agentv1.GetCARequest{})
	if err != nil || len(ca.GetCaPem()) == 0 {
		t.Fatalf("GetCA err = %v", err)
	}
}

func TestRenewFromPeerCN(t *testing.T) {
	srv, svc := newServer(t)
	registerNode(t, srv, svc, "node-1")

	ctx := peer.NewContext(context.Background(), &peer.Peer{
		AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{{Subject: pkix.Name{CommonName: "node-1"}}},
		}},
	})
	resp, err := srv.Renew(ctx, &agentv1.RenewRequest{CsrPem: newCSR(t)})
	if err != nil || len(resp.GetCertChainPem()) == 0 {
		t.Fatalf("renew err = %v", err)
	}

	if _, err := srv.Renew(context.Background(), &agentv1.RenewRequest{CsrPem: newCSR(t)}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("无客户端证书应 Unauthenticated, got %v", status.Code(err))
	}
}

func TestErrorMapping(t *testing.T) {
	srv, _ := newServer(t)
	_, err := srv.Register(context.Background(), &agentv1.RegisterRequest{Token: "bad", NodeId: "n", CsrPem: newCSR(t)})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("无效 Token 应 Unauthenticated, got %v", status.Code(err))
	}
}

// BE-29：NewGRPCServer 启动后经 bufconn 可完成注册。
func TestNewGRPCServerServes(t *testing.T) {
	_, svc := newServer(t)
	lis := bufconn.Listen(1024 * 1024)
	srv, err := registration.NewGRPCServer(config.RegistrationConf{Insecure: true}, svc)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := agentv1.NewRegistrationServiceClient(conn)
	token, err := svc.CreateToken(context.Background(), "entry")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Register(context.Background(), &agentv1.RegisterRequest{
		Token: token, NodeId: "node-1", NodeType: agentv1.AgentNodeType_AGENT_NODE_TYPE_ENTRY, CsrPem: newCSR(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetCertChainPem()) == 0 {
		t.Fatal("未返回证书链")
	}
}
