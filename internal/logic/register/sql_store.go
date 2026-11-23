package register

import (
	"context"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// epoch 用作可空时间列的哨兵（COALESCE 后判断）。
var epoch = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

// SQLStore 是 Store 的 PostgreSQL 实现（仅使用 go-zero sqlx）。
type SQLStore struct {
	db sqlx.SqlConn
}

// NewSQLStore 构造 PostgreSQL Store。
func NewSQLStore(db sqlx.SqlConn) *SQLStore { return &SQLStore{db: db} }

func (s *SQLStore) CreateToken(ctx context.Context, rec TokenRecord) error {
	_, err := s.db.ExecCtx(ctx,
		`INSERT INTO node_registration_tokens (token_hash, node_type, expires_at) VALUES ($1, $2, $3)`,
		rec.Hash, rec.NodeType, rec.ExpiresAt)
	return err
}

type tokenRow struct {
	Hash      string    `db:"token_hash"`
	NodeType  string    `db:"node_type"`
	ExpiresAt time.Time `db:"expires_at"`
	UsedAt    time.Time `db:"used_at"`
}

func (s *SQLStore) GetToken(ctx context.Context, hash string) (TokenRecord, bool, error) {
	var row tokenRow
	err := s.db.QueryRowCtx(ctx, &row,
		`SELECT token_hash, node_type, expires_at,
		        COALESCE(used_at, 'epoch'::timestamptz) AS used_at
		 FROM node_registration_tokens WHERE token_hash = $1`, hash)
	if errors.Is(err, sqlx.ErrNotFound) {
		return TokenRecord{}, false, nil
	}
	if err != nil {
		return TokenRecord{}, false, err
	}
	rec := TokenRecord{Hash: row.Hash, NodeType: row.NodeType, ExpiresAt: row.ExpiresAt}
	if row.UsedAt.After(epoch) {
		rec.UsedAt = row.UsedAt
	}
	return rec, true, nil
}

func (s *SQLStore) MarkTokenUsed(ctx context.Context, hash string, at time.Time) error {
	_, err := s.db.ExecCtx(ctx,
		`UPDATE node_registration_tokens SET used_at = $2 WHERE token_hash = $1`, hash, at)
	return err
}

type certRow struct {
	NodeID      string    `db:"node_id"`
	Fingerprint string    `db:"fingerprint_sha256"`
	NotAfter    time.Time `db:"not_after"`
	RevokedAt   time.Time `db:"revoked_at"`
}

func (s *SQLStore) GetCert(ctx context.Context, nodeID string) (CertRecord, bool, error) {
	var row certRow
	err := s.db.QueryRowCtx(ctx, &row,
		`SELECT node_id, fingerprint_sha256, not_after,
		        COALESCE(revoked_at, 'epoch'::timestamptz) AS revoked_at
		 FROM node_certificates WHERE node_id = $1`, nodeID)
	if errors.Is(err, sqlx.ErrNotFound) {
		return CertRecord{}, false, nil
	}
	if err != nil {
		return CertRecord{}, false, err
	}
	rec := CertRecord{NodeID: row.NodeID, Fingerprint: row.Fingerprint, NotAfter: row.NotAfter}
	if row.RevokedAt.After(epoch) {
		rec.RevokedAt = row.RevokedAt
	}
	return rec, true, nil
}

func (s *SQLStore) SaveCert(ctx context.Context, rec CertRecord) error {
	_, err := s.db.ExecCtx(ctx,
		`INSERT INTO node_certificates (node_id, fingerprint_sha256, not_after)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (node_id) DO UPDATE SET
		   fingerprint_sha256 = EXCLUDED.fingerprint_sha256,
		   not_after = EXCLUDED.not_after,
		   revoked_at = NULL,
		   issued_at = now()`,
		rec.NodeID, rec.Fingerprint, rec.NotAfter)
	return err
}

func (s *SQLStore) RevokeCert(ctx context.Context, nodeID string, at time.Time) error {
	_, err := s.db.ExecCtx(ctx,
		`UPDATE node_certificates SET revoked_at = $2 WHERE node_id = $1`, nodeID, at)
	return err
}

var _ Store = (*SQLStore)(nil)
