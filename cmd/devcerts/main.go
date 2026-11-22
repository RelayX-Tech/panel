// Command devcerts 生成开发 / 联调所需的 Panel CA、mTLS 证书、Ed25519 密钥与主密钥（仅开发使用）。
package main

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

func main() {
	out := flag.String("out", "dev/deploy", "输出目录")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "devcerts:", err)
		os.Exit(1)
	}
	fmt.Println("已生成开发凭据到", *out)
}

func run(out string) error {
	if err := os.MkdirAll(out, 0o700); err != nil {
		return err
	}
	caCert, caKey, err := newCA()
	if err != nil {
		return err
	}
	if err := writePEM(filepath.Join(out, "panel-ca.crt"), "CERTIFICATE", caCert.Raw); err != nil {
		return err
	}
	if err := writePEM(filepath.Join(out, "panel-ca.key"), "PRIVATE KEY", marshalKey(caKey)); err != nil {
		return err
	}
	if err := writeLeaf(filepath.Join(out, "registration"), caCert, caKey, "relayx-registration", []string{"relayx-registration"}, []string{"127.0.0.1"}, true); err != nil {
		return err
	}
	if err := writeLeaf(filepath.Join(out, "panel-client"), caCert, caKey, "panel", nil, nil, true); err != nil {
		return err
	}
	if err := writeEd25519(out); err != nil {
		return err
	}
	return writeMasterKey(out)
}

func newCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "relayx-panel-ca"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(5 * 365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	return cert, key, nil
}

func writeLeaf(prefix string, ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn string, dns []string, ips []string, bothEKU bool) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	eku := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	if bothEKU {
		eku = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn},
		DNSNames:              dns,
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(90 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           eku,
		BasicConstraintsValid: true,
	}
	for _, ip := range ips {
		tmpl.IPAddresses = append(tmpl.IPAddresses, net.ParseIP(ip))
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		return err
	}
	if err := writePEM(prefix+".crt", "CERTIFICATE", der); err != nil {
		return err
	}
	return writePEM(prefix+".key", "PRIVATE KEY", marshalKey(key))
}

func writeEd25519(out string) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "panel-ed25519.pub"), []byte(base64.StdEncoding.EncodeToString(pub)), 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "panel-ed25519.key"), []byte(base64.StdEncoding.EncodeToString(priv)), 0o600)
}

func writeMasterKey(out string) error {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "master-key.txt"), []byte(base64.StdEncoding.EncodeToString(key)), 0o600)
}

func marshalKey(key any) []byte {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil
	}
	return der
}

func writePEM(path, blockType string, der []byte) error {
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}), 0o600)
}
