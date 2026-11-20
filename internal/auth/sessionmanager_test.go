package auth

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/model"
)

type touchRecord struct {
	ID        string
	LastSeen  time.Time
	ExpiresAt time.Time
}

type fakeSessionModel struct {
	rows    map[string]*model.SessionWithUser
	touches []touchRecord
	revoked []string
}

func newFakeSessionModel() *fakeSessionModel {
	return &fakeSessionModel{rows: map[string]*model.SessionWithUser{}}
}

func (f *fakeSessionModel) Create(_ context.Context, s *model.Session) error {
	s.ID = "sess-1"
	f.rows[string(s.SessionHash)] = &model.SessionWithUser{
		Session: *s,
		User:    model.User{ID: s.UserID, Account: "admin"},
	}
	return nil
}

func (f *fakeSessionModel) FindByHash(_ context.Context, hash []byte) (*model.SessionWithUser, error) {
	row, ok := f.rows[string(hash)]
	if !ok {
		return nil, model.ErrNotFound
	}
	cp := *row
	return &cp, nil
}

func (f *fakeSessionModel) Touch(_ context.Context, id string, lastSeen, expiresAt time.Time) error {
	f.touches = append(f.touches, touchRecord{ID: id, LastSeen: lastSeen, ExpiresAt: expiresAt})
	for _, row := range f.rows {
		if row.Session.ID == id {
			row.Session.LastSeenAt = lastSeen
			row.Session.ExpiresAt = expiresAt
		}
	}
	return nil
}

func (f *fakeSessionModel) RevokeByHash(_ context.Context, hash []byte) error {
	f.revoked = append(f.revoked, string(hash))
	return nil
}

func (f *fakeSessionModel) DeleteExpiredBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func newTestManager(sessions *fakeSessionModel, now time.Time) *SessionManager {
	m := NewSessionManager(sessions, 24*time.Hour, 168*time.Hour, 5*time.Minute)
	m.Now = func() time.Time { return now }
	return m
}

func TestSessionManagerCreateStoresOnlyHash(t *testing.T) {
	f := newFakeSessionModel()
	now := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	m := newTestManager(f, now)

	token, sess, err := m.Create(context.Background(), "user-1", "10.0.0.1", "ua")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if token == "" || len(token) != 43 {
		t.Fatalf("token = %q", token)
	}
	if bytes.Contains(sess.SessionHash, []byte(token)) {
		t.Fatal("session hash contains plaintext token")
	}
	if !bytes.Equal(sess.SessionHash, HashSessionToken(token)) {
		t.Error("stored hash is not sha256(token)")
	}
	if !sess.ExpiresAt.Equal(now.Add(24 * time.Hour)) {
		t.Errorf("expiresAt = %v", sess.ExpiresAt)
	}
	if !sess.AbsoluteExpiresAt.Equal(now.Add(168 * time.Hour)) {
		t.Errorf("absoluteExpiresAt = %v", sess.AbsoluteExpiresAt)
	}
}

func TestSessionManagerAuthenticate(t *testing.T) {
	f := newFakeSessionModel()
	now := time.Now().UTC()
	m := newTestManager(f, now)
	token, _, _ := m.Create(context.Background(), "user-1", "", "")

	got, err := m.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if got.User.ID != "user-1" {
		t.Errorf("user = %+v", got.User)
	}

	if _, err := m.Authenticate(context.Background(), "unknown-token"); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("missing session error = %v", err)
	}
}

func TestSessionManagerAuthenticateRejectsInvalidStates(t *testing.T) {
	cases := map[string]func(*model.SessionWithUser){
		"expired":       func(s *model.SessionWithUser) { s.Session.ExpiresAt = time.Now().Add(-time.Minute) },
		"absolute":      func(s *model.SessionWithUser) { s.Session.AbsoluteExpiresAt = time.Now().Add(-time.Minute) },
		"revoked":       func(s *model.SessionWithUser) { r := time.Now(); s.Session.RevokedAt = &r },
		"user disabled": func(s *model.SessionWithUser) { d := time.Now(); s.User.DisabledAt = &d },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeSessionModel()
			now := time.Now().UTC()
			m := newTestManager(f, now)
			token, _, _ := m.Create(context.Background(), "user-1", "", "")
			row := f.rows[string(HashSessionToken(token))]
			mutate(row)

			if _, err := m.Authenticate(context.Background(), token); !errors.Is(err, ErrSessionExpired) {
				t.Errorf("error = %v, want ErrSessionExpired", err)
			}
		})
	}
}

func TestSessionManagerTouchesAfterInterval(t *testing.T) {
	f := newFakeSessionModel()
	now := time.Now().UTC()
	m := newTestManager(f, now)
	token, _, _ := m.Create(context.Background(), "user-1", "", "")

	// 刚创建：不 touch
	if _, err := m.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if len(f.touches) != 0 {
		t.Fatalf("touched fresh session %d times", len(f.touches))
	}

	// 6 分钟后：touch 一次，expiresAt = min(now+24h, absolute)
	later := now.Add(6 * time.Minute)
	m.Now = func() time.Time { return later }
	if _, err := m.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("authenticate later: %v", err)
	}
	if len(f.touches) != 1 {
		t.Fatalf("touches = %d, want 1", len(f.touches))
	}
	if !f.touches[0].ExpiresAt.Equal(later.Add(24 * time.Hour)) {
		t.Errorf("touched expiry = %v", f.touches[0].ExpiresAt)
	}

	// 接近绝对上限（144h30m）：续期不得突破 absolute
	nearAbsolute := now.Add(144*time.Hour + 30*time.Minute)
	row := f.rows[string(HashSessionToken(token))]
	row.Session.LastSeenAt = nearAbsolute.Add(-6 * time.Minute)
	row.Session.ExpiresAt = nearAbsolute.Add(time.Hour)
	m.Now = func() time.Time { return nearAbsolute }
	if _, err := m.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("authenticate near absolute: %v", err)
	}
	if got := f.touches[len(f.touches)-1].ExpiresAt; !got.Equal(now.Add(168 * time.Hour)) {
		t.Errorf("expiry = %v, want capped at absolute", got)
	}

	// 绝对上限前 1 小时仍有效
	m.Now = func() time.Time { return now.Add(167 * time.Hour) }
	if _, err := m.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("authenticate before absolute: %v", err)
	}
}

func TestSessionManagerRevoke(t *testing.T) {
	f := newFakeSessionModel()
	m := newTestManager(f, time.Now().UTC())
	token, _, _ := m.Create(context.Background(), "user-1", "", "")

	if err := m.Revoke(context.Background(), token); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if len(f.revoked) != 1 || f.revoked[0] != string(HashSessionToken(token)) {
		t.Errorf("revoked = %x", f.revoked)
	}
}

func TestSessionContextHelpers(t *testing.T) {
	row := &model.SessionWithUser{User: model.User{ID: "u1"}}
	ctx := ContextWithSession(context.Background(), row)
	if got := SessionFromContext(ctx); got == nil || got.User.ID != "u1" {
		t.Errorf("session from context = %+v", got)
	}
	if SessionFromContext(context.Background()) != nil {
		t.Error("empty context should not return a session")
	}
}
