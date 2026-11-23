// Package pki 实现 Panel CA：加载 CA、校验证书请求（CSR）并签发客户端证书（design §3.10）。
package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"
)

var (
	// ErrCSRInvalid 表示 CSR 解析 / 签名 / 密钥类型非法。
	ErrCSRInvalid = errors.New("csr invalid")
	// ErrCertIssue 表示 CA 签发失败。
	ErrCertIssue = errors.New("certificate issue failed")
)

// CA 是 Panel 证书颁发机构。
type CA struct {
	cert *x509.Certificate
	key  crypto.Signer
	pem  []byte
}

// Issued 是签发结果。
type Issued struct {
	CertChainPEM []byte
	NotAfter     time.Time
	Fingerprint  string
}

// LoadCA 从 PEM 文件加载 CA 证书与私钥。
func LoadCA(certFile, keyFile string) (*CA, error) {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("读取 CA 证书: %w", err)
	}
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, errors.New("CA 证书 PEM 解析失败")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析 CA 证书: %w", err)
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("读取 CA 私钥: %w", err)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, errors.New("CA 私钥 PEM 解析失败")
	}
	key, err := parsePrivateKey(keyBlock)
	if err != nil {
		return nil, err
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, errors.New("CA 私钥不可用于签名")
	}
	return &CA{cert: cert, key: signer, pem: certPEM}, nil
}

// NewCA 用已有证书与签名者构造 CA（测试 / 内存使用）。
func NewCA(cert *x509.Certificate, key crypto.Signer, certPEM []byte) *CA {
	return &CA{cert: cert, key: key, pem: certPEM}
}

// CertificatePEM 返回 CA 证书 PEM（供 GetCA）。
func (ca *CA) CertificatePEM() []byte { return ca.pem }

// SignCSR 校验 CSR 并由 CA 签发客户端证书（CN/SAN=nodeID、EKU=clientAuth）。
func (ca *CA) SignCSR(csrPEM []byte, nodeID string, validity time.Duration) (*Issued, error) {
	block, _ := pem.Decode(csrPEM)
	if block == nil {
		return nil, ErrCSRInvalid
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, ErrCSRInvalid
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, ErrCSRInvalid
	}
	if !allowedKey(csr.PublicKey) {
		return nil, ErrCSRInvalid
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: nodeID},
		DNSNames:     []string{nodeID},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(validity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		// Agent 既是 Panel 调用的服务端（serverAuth），也用于注册客户端（clientAuth）。
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, csr.PublicKey, ca.key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCertIssue, err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	chain := make([]byte, 0, len(certPEM)+len(ca.pem))
	chain = append(chain, certPEM...)
	chain = append(chain, ca.pem...)

	sum := sha256.Sum256(der)
	return &Issued{
		CertChainPEM: chain,
		NotAfter:     tmpl.NotAfter,
		Fingerprint:  hex.EncodeToString(sum[:]),
	}, nil
}

func allowedKey(pub crypto.PublicKey) bool {
	switch pub.(type) {
	case *ecdsa.PublicKey, ed25519.PublicKey:
		return true
	default:
		return false
	}
}

func parsePrivateKey(block *pem.Block) (any, error) {
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}
