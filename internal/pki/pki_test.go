package pki_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/pki"
)

func newCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
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
	return cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
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

func TestSignCSR(t *testing.T) {
	cert, key, caPEM := newCA(t)
	ca := pki.NewCA(cert, key, caPEM)

	issued, err := ca.SignCSR(newCSR(t), "node-1", 90*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(issued.CertChainPEM)
	if block == nil {
		t.Fatal("证书链 PEM 解析失败")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if leaf.Subject.CommonName != "node-1" {
		t.Fatalf("CN = %q", leaf.Subject.CommonName)
	}
	if len(leaf.DNSNames) != 1 || leaf.DNSNames[0] != "node-1" {
		t.Fatalf("SAN = %v", leaf.DNSNames)
	}
	hasClientAuth := false
	for _, eku := range leaf.ExtKeyUsage {
		if eku == x509.ExtKeyUsageClientAuth {
			hasClientAuth = true
		}
	}
	if !hasClientAuth {
		t.Fatal("缺少 clientAuth EKU")
	}
	if validFor := time.Until(leaf.NotAfter); validFor < 89*24*time.Hour || validFor > 91*24*time.Hour {
		t.Fatalf("有效期异常: %v", validFor)
	}
	if len(issued.Fingerprint) != 64 {
		t.Fatalf("指纹长度 = %d", len(issued.Fingerprint))
	}

	pool := x509.NewCertPool()
	pool.AddCert(cert)
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: pool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		t.Fatalf("证书链校验失败: %v", err)
	}
}

func TestSignCSRInvalid(t *testing.T) {
	cert, key, caPEM := newCA(t)
	ca := pki.NewCA(cert, key, caPEM)

	if _, err := ca.SignCSR([]byte("not a pem"), "node-1", time.Hour); !errors.Is(err, pki.ErrCSRInvalid) {
		t.Fatalf("非 PEM 应 ErrCSRInvalid, got %v", err)
	}
	if _, err := ca.SignCSR([]byte("-----BEGIN CERTIFICATE REQUEST-----\nAAAA\n-----END CERTIFICATE REQUEST-----\n"), "node-1", time.Hour); !errors.Is(err, pki.ErrCSRInvalid) {
		t.Fatalf("内容非法应 ErrCSRInvalid, got %v", err)
	}
}
