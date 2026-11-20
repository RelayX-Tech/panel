package response

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func authCfg(secure bool) CookieConfig {
	return CookieConfig{Name: "relayx_session", Secure: secure, MaxAgeSeconds: 604800}
}

func TestSetSessionCookieAttributes(t *testing.T) {
	rec := httptest.NewRecorder()
	SetSessionCookie(rec, authCfg(true), "token-value")

	c := rec.Result().Cookies()
	if len(c) != 1 {
		t.Fatalf("cookies = %d, want 1", len(c))
	}
	ck := c[0]
	if ck.Name != "relayx_session" || ck.Value != "token-value" {
		t.Errorf("cookie = %s=%s", ck.Name, ck.Value)
	}
	if !ck.HttpOnly || !ck.Secure || ck.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie flags = %+v", ck)
	}
	if ck.Path != "/" {
		t.Errorf("path = %q, want /", ck.Path)
	}
	if ck.MaxAge != 604800 {
		t.Errorf("maxAge = %d, want 604800", ck.MaxAge)
	}
	if ck.Domain != "" {
		t.Errorf("domain = %q, want host-only", ck.Domain)
	}
}

func TestSetSessionCookieOmitsSecureInDev(t *testing.T) {
	rec := httptest.NewRecorder()
	SetSessionCookie(rec, authCfg(false), "token-value")
	if rec.Result().Cookies()[0].Secure {
		t.Error("Secure set although CookieSecure=false")
	}
}

func TestClearSessionCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	ClearSessionCookie(rec, authCfg(true))

	ck := rec.Result().Cookies()[0]
	if ck.MaxAge != 0 && ck.MaxAge != -1 {
		t.Errorf("maxAge = %d, want immediate expiry", ck.MaxAge)
	}
	if !strings.Contains(rec.Header().Get("Set-Cookie"), "relayx_session=") {
		t.Errorf("Set-Cookie = %q", rec.Header().Get("Set-Cookie"))
	}
	if !ck.HttpOnly || !ck.Secure {
		t.Errorf("clear cookie flags = %+v", ck)
	}
}

func TestReadSessionCookie(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: "relayx_session", Value: "abc"})
	if got := ReadSessionCookie(r, "relayx_session"); got != "abc" {
		t.Errorf("cookie = %q, want abc", got)
	}
	if got := ReadSessionCookie(httptest.NewRequest("GET", "/", nil), "relayx_session"); got != "" {
		t.Errorf("missing cookie = %q, want empty", got)
	}
}
