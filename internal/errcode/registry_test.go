package errcode

import (
	"testing"
)

func TestLoadRegistry(t *testing.T) {
	codes, err := LoadFile("../../api/error-codes.yaml")
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	if len(codes) < 12 {
		t.Fatalf("codes = %d, want at least 12", len(codes))
	}

	seen := map[string]bool{}
	for _, c := range codes {
		if seen[c.Code] {
			t.Errorf("duplicate code %q", c.Code)
		}
		seen[c.Code] = true
		if err := c.Validate(); err != nil {
			t.Errorf("code %q: %v", c.Code, err)
		}
		if c.I18n != "errors."+c.Code {
			t.Errorf("code %q i18n = %q, want errors.%s", c.Code, c.I18n, c.Code)
		}
	}
}

func TestValidateRejectsInvalidRegistry(t *testing.T) {
	valid := Code{Code: "AUTH_UNAUTHORIZED", HTTP: 401, I18n: "errors.AUTH_UNAUTHORIZED", Owner: "backend-panel", Since: "2026-11-19", Message: "未认证"}

	cases := map[string]Code{
		"empty code":        {HTTP: 401, I18n: "errors.X", Owner: "o", Since: "2026-11-19", Message: "m"},
		"invalid http":      {Code: "AUTH_UNAUTHORIZED", HTTP: 418, I18n: "errors.AUTH_UNAUTHORIZED", Owner: "o", Since: "2026-11-19", Message: "m"},
		"missing owner":     {Code: "AUTH_UNAUTHORIZED", HTTP: 401, I18n: "errors.AUTH_UNAUTHORIZED", Since: "2026-11-19", Message: "m"},
		"i18n mismatch":     {Code: "AUTH_UNAUTHORIZED", HTTP: 401, I18n: "errors.WRONG", Owner: "o", Since: "2026-11-19", Message: "m"},
		"missing since":     {Code: "AUTH_UNAUTHORIZED", HTTP: 401, I18n: "errors.AUTH_UNAUTHORIZED", Owner: "o", Message: "m"},
		"empty message":     {Code: "AUTH_UNAUTHORIZED", HTTP: 401, I18n: "errors.AUTH_UNAUTHORIZED", Owner: "o", Since: "2026-11-19"},
		"deprecated no alt": {Code: "AUTH_UNAUTHORIZED", HTTP: 401, I18n: "errors.AUTH_UNAUTHORIZED", Owner: "o", Since: "2026-11-19", Message: "m", Deprecated: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if err := c.Validate(); err == nil {
				t.Fatalf("code %+v should be rejected", c)
			}
		})
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("valid code rejected: %v", err)
	}
}

func TestValidateAllRejectsDuplicate(t *testing.T) {
	c := Code{Code: "AUTH_UNAUTHORIZED", HTTP: 401, I18n: "errors.AUTH_UNAUTHORIZED", Owner: "o", Since: "2026-11-19", Message: "m"}
	if err := ValidateAll([]Code{c, c}); err == nil {
		t.Fatal("duplicate codes should be rejected")
	}
}
