package register_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/logic/register"
	"github.com/relayx-tech/panel/internal/pki"
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

func newService(t *testing.T) *register.Service {
	t.Helper()
	return register.New(register.NewMemoryStore(), newCA(t))
}

func TestRegisterFlow(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()

	token, err := svc.CreateToken(ctx, "entry")
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Register(ctx, token, "node-1", "entry", newCSR(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CertChainPEM) == 0 || len(res.CaPEM) == 0 || len(res.Fingerprint) != 64 {
		t.Fatalf("result = %+v", res)
	}

	// 单次 Token 复用
	if _, err := svc.Register(ctx, token, "node-2", "entry", newCSR(t)); !errors.Is(err, register.ErrTokenUsed) {
		t.Fatalf("复用 Token err = %v", err)
	}

	// 同节点重复注册
	token2, _ := svc.CreateToken(ctx, "entry")
	if _, err := svc.Register(ctx, token2, "node-1", "entry", newCSR(t)); !errors.Is(err, register.ErrAlreadyRegistered) {
		t.Fatalf("重复注册 err = %v", err)
	}
}

func TestRegisterTokenErrors(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()

	if _, err := svc.Register(ctx, "bad-token", "node-1", "entry", newCSR(t)); !errors.Is(err, register.ErrTokenInvalid) {
		t.Fatalf("无效 Token err = %v", err)
	}

	tunnelToken, _ := svc.CreateToken(ctx, "tunnel")
	if _, err := svc.Register(ctx, tunnelToken, "node-1", "entry", newCSR(t)); !errors.Is(err, register.ErrNodeTypeMismatch) {
		t.Fatalf("类型不符 err = %v", err)
	}

	now := time.Unix(1000, 0)
	svc.SetClock(func() time.Time { return now })
	expiring, _ := svc.CreateToken(ctx, "entry")
	svc.SetClock(func() time.Time { return now.Add(25 * time.Hour) })
	if _, err := svc.Register(ctx, expiring, "node-1", "entry", newCSR(t)); !errors.Is(err, register.ErrTokenExpired) {
		t.Fatalf("过期 Token err = %v", err)
	}

	fresh, _ := svc.CreateToken(ctx, "entry")
	if _, err := svc.Register(ctx, fresh, "node-2", "entry", []byte("bad-csr")); !errors.Is(err, register.ErrCSRInvalid) {
		t.Fatalf("非法 CSR err = %v", err)
	}
}

// REG-13：错误信息不泄露 Token / CSR 明文。
func TestRegisterDoesNotLeakSecrets(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()

	const token = "super-secret-token-value"
	if _, err := svc.Register(ctx, token, "node-1", "entry", newCSR(t)); err == nil || strings.Contains(err.Error(), token) {
		t.Fatalf("错误信息不应泄露 token: %v", err)
	}

	fresh, _ := svc.CreateToken(ctx, "entry")
	if _, err := svc.Register(ctx, fresh, "node-2", "entry", []byte("SECRET_CSR_BYTES")); err == nil || strings.Contains(err.Error(), "SECRET_CSR_BYTES") {
		t.Fatalf("错误信息不应泄露 CSR: %v", err)
	}
}

// REG-16：已注册节点的指纹变更需先吊销，不能直接覆盖。
func TestRegisterFingerprintChangeRequiresRevoke(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()

	token, _ := svc.CreateToken(ctx, "entry")
	if _, err := svc.Register(ctx, token, "node-1", "entry", newCSR(t)); err != nil {
		t.Fatal(err)
	}
	token2, _ := svc.CreateToken(ctx, "entry")
	if _, err := svc.Register(ctx, token2, "node-1", "entry", newCSR(t)); !errors.Is(err, register.ErrAlreadyRegistered) {
		t.Fatalf("指纹变更需先吊销, got %v", err)
	}
	if err := svc.Revoke(ctx, "node-1"); err != nil {
		t.Fatal(err)
	}
	token3, _ := svc.CreateToken(ctx, "entry")
	if _, err := svc.Register(ctx, token3, "node-1", "entry", newCSR(t)); err != nil {
		t.Fatalf("吊销后应可重注册: %v", err)
	}
}

func TestRenewAndRevoke(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()

	token, _ := svc.CreateToken(ctx, "entry")
	if _, err := svc.Register(ctx, token, "node-1", "entry", newCSR(t)); err != nil {
		t.Fatal(err)
	}
	if res, err := svc.Renew(ctx, "node-1", newCSR(t)); err != nil || len(res.CertChainPEM) == 0 {
		t.Fatalf("renew = %+v, err = %v", res, err)
	}

	if err := svc.Revoke(ctx, "node-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Renew(ctx, "node-1", newCSR(t)); !errors.Is(err, register.ErrCertRevoked) {
		t.Fatalf("吊销后 renew err = %v", err)
	}
	if _, err := svc.Renew(ctx, "unknown", newCSR(t)); !errors.Is(err, register.ErrNodeNotFound) {
		t.Fatalf("未知节点 renew err = %v", err)
	}
}
