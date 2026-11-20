package requestid

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

var ulidRe = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

func TestMiddlewareGeneratesULIDWhenAbsent(t *testing.T) {
	var seen string
	h := Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = FromContext(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !ulidRe.MatchString(seen) {
		t.Errorf("context requestId = %q, want ULID", seen)
	}
	if got := rec.Header().Get("X-Request-Id"); got != seen {
		t.Errorf("response header = %q, want %q", got, seen)
	}
}

func TestMiddlewareAcceptsValidULID(t *testing.T) {
	const given = "01J8Z6YQ9K3N4R5T6V7W8X9Y0Z"
	var seen string
	h := Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", given)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen != given {
		t.Errorf("context requestId = %q, want %q", seen, given)
	}
}

func TestMiddlewareAcceptsUUID(t *testing.T) {
	const given = "0b6f1c1e-3a4d-4f0e-9a11-6d2c7b8e9f01"
	var seen string
	h := Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", given)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen != given {
		t.Errorf("context requestId = %q, want %q", seen, given)
	}
	if got := rec.Header().Get("X-Request-Id"); got != given {
		t.Errorf("response header = %q, want %q", got, given)
	}
}

func TestNormalizeAcceptsUUIDAndULID(t *testing.T) {
	cases := []string{
		"01J8Z6YQ9K3N4R5T6V7W8X9Y0Z",
		"0b6f1c1e-3a4d-4f0e-9a11-6d2c7b8e9f01",
		"0B6F1C1E-3A4D-4F0E-9A11-6D2C7B8E9F01",
	}
	for _, tc := range cases {
		if got := Normalize(tc); got == "" {
			t.Errorf("Normalize(%q) rejected a valid id", tc)
		}
	}
}

func TestMiddlewareRejectsMalformedRequestID(t *testing.T) {
	var seen string
	h := Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = FromContext(r.Context())
	}))

	for _, bad := range []string{"not-a-ulid", "../../etc/passwd", "01J8Z6YQ9K3N4R5T6V7W8X9Y0", "01J8Z6YQ9K3N4R5T6V7W8X9Y0ZZ"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-Id", bad)
		h.ServeHTTP(httptest.NewRecorder(), req)
		if seen == bad {
			t.Errorf("malformed request id %q was accepted", bad)
		}
	}
}

func TestGenerateProducesUniqueULIDs(t *testing.T) {
	a, b := Generate(), Generate()
	if a == b {
		t.Errorf("two generated ULIDs are identical: %q", a)
	}
	if len(a) != 26 || !ulidRe.MatchString(a) {
		t.Errorf("generated id %q is not a ULID", a)
	}
}
