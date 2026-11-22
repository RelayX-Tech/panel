// Package register 实现节点注册（Token 生命周期 + CSR 签发 + 续期 / 吊销，design §3.10）。
package register

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/relayx-tech/panel/internal/pki"
)

// 稳定错误（由调用方映射为 gRPC / HTTP 错误码）。
var (
	ErrTokenInvalid      = errors.New("registration token invalid")
	ErrTokenExpired      = errors.New("registration token expired")
	ErrTokenUsed         = errors.New("registration token already used")
	ErrNodeTypeMismatch  = errors.New("node type mismatch")
	ErrAlreadyRegistered = errors.New("node already registered")
	ErrCSRInvalid        = errors.New("csr invalid")
	ErrCertIssue         = errors.New("certificate issue failed")
	ErrCertRevoked       = errors.New("certificate revoked")
	ErrNodeNotFound      = errors.New("node not found")
)

// TokenRecord 是注册 Token 的持久记录（明文不落库）。
type TokenRecord struct {
	Hash      string
	NodeType  string
	ExpiresAt time.Time
	UsedAt    time.Time
}

// CertRecord 是节点客户端证书记录。
type CertRecord struct {
	NodeID      string
	Serial      string
	Fingerprint string
	NotAfter    time.Time
	RevokedAt   time.Time
}

// Store 是注册持久化抽象（真实实现走 PostgreSQL；测试用内存实现）。
type Store interface {
	CreateToken(ctx context.Context, rec TokenRecord) error
	GetToken(ctx context.Context, hash string) (TokenRecord, bool, error)
	MarkTokenUsed(ctx context.Context, hash string, at time.Time) error
	GetCert(ctx context.Context, nodeID string) (CertRecord, bool, error)
	SaveCert(ctx context.Context, rec CertRecord) error
	RevokeCert(ctx context.Context, nodeID string, at time.Time) error
}

// Result 是签发结果。
type Result struct {
	CertChainPEM []byte
	CaPEM        []byte
	NotAfter     time.Time
	Fingerprint  string
}

// Service 编排注册流程。
type Service struct {
	store    Store
	ca       *pki.CA
	now      func() time.Time
	validity time.Duration
	tokenTTL time.Duration
}

// New 构造注册服务（证书 90 天、Token 24h）。
func New(store Store, ca *pki.CA) *Service {
	return &Service{store: store, ca: ca, now: time.Now, validity: 90 * 24 * time.Hour, tokenTTL: 24 * time.Hour}
}

// SetClock 覆盖时钟（测试用）。
func (s *Service) SetClock(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

// GetCA 返回 CA 证书 PEM。
func (s *Service) GetCA() []byte { return s.ca.CertificatePEM() }

// CreateToken 生成一次性明文 Token（仅此一次返回），持久化其哈希。
func (s *Service) CreateToken(ctx context.Context, nodeType string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	plaintext := base64.RawURLEncoding.EncodeToString(raw)
	rec := TokenRecord{Hash: hashToken(plaintext), NodeType: nodeType, ExpiresAt: s.now().Add(s.tokenTTL)}
	if err := s.store.CreateToken(ctx, rec); err != nil {
		return "", err
	}
	return plaintext, nil
}

// Register 用一次性 Token + CSR 换取客户端证书。
func (s *Service) Register(ctx context.Context, plaintext, nodeID, nodeType string, csrPEM []byte) (*Result, error) {
	rec, ok, err := s.store.GetToken(ctx, hashToken(plaintext))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrTokenInvalid
	}
	now := s.now()
	if !rec.UsedAt.IsZero() {
		return nil, ErrTokenUsed
	}
	if now.After(rec.ExpiresAt) {
		return nil, ErrTokenExpired
	}
	if rec.NodeType != "" && rec.NodeType != nodeType {
		return nil, ErrNodeTypeMismatch
	}
	if cert, ok, _ := s.store.GetCert(ctx, nodeID); ok && cert.RevokedAt.IsZero() {
		return nil, ErrAlreadyRegistered
	}

	issued, err := s.sign(csrPEM, nodeID)
	if err != nil {
		return nil, err
	}
	if err := s.store.MarkTokenUsed(ctx, rec.Hash, now); err != nil {
		return nil, err
	}
	if err := s.store.SaveCert(ctx, certRecord(nodeID, issued)); err != nil {
		return nil, err
	}
	return s.result(issued), nil
}

// Renew 为已认证节点换发新证书（证书已吊销则拒绝）。
func (s *Service) Renew(ctx context.Context, nodeID string, csrPEM []byte) (*Result, error) {
	cert, ok, err := s.store.GetCert(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNodeNotFound
	}
	if !cert.RevokedAt.IsZero() {
		return nil, ErrCertRevoked
	}
	issued, err := s.sign(csrPEM, nodeID)
	if err != nil {
		return nil, err
	}
	if err := s.store.SaveCert(ctx, certRecord(nodeID, issued)); err != nil {
		return nil, err
	}
	return s.result(issued), nil
}

// Revoke 吊销节点证书。
func (s *Service) Revoke(ctx context.Context, nodeID string) error {
	return s.store.RevokeCert(ctx, nodeID, s.now())
}

func (s *Service) sign(csrPEM []byte, nodeID string) (*pki.Issued, error) {
	issued, err := s.ca.SignCSR(csrPEM, nodeID, s.validity)
	if err != nil {
		if errors.Is(err, pki.ErrCSRInvalid) {
			return nil, ErrCSRInvalid
		}
		return nil, ErrCertIssue
	}
	return issued, nil
}

func (s *Service) result(issued *pki.Issued) *Result {
	return &Result{
		CertChainPEM: issued.CertChainPEM,
		CaPEM:        s.ca.CertificatePEM(),
		NotAfter:     issued.NotAfter,
		Fingerprint:  issued.Fingerprint,
	}
}

func certRecord(nodeID string, issued *pki.Issued) CertRecord {
	return CertRecord{NodeID: nodeID, Fingerprint: issued.Fingerprint, NotAfter: issued.NotAfter}
}

func hashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
