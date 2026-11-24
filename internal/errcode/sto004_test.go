package errcode_test

import (
	"testing"

	"github.com/relayx-tech/panel/internal/errcode"
)

// TestSTO004CodesRegistered 校验 design.md §3.6 新增错误码已注册且 HTTP 状态一致（BE-API-26）。
func TestSTO004CodesRegistered(t *testing.T) {
	codes, err := errcode.LoadFile("../../api/error-codes.yaml")
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	byCode := make(map[string]errcode.Code, len(codes))
	for _, c := range codes {
		byCode[c.Code] = c
	}

	want := map[string]int{
		"AUTH_FORBIDDEN":                    403,
		"FORBIDDEN_SELF_SESSION_REVOKE":     403,
		"NOT_FOUND_ALERT":                   404,
		"NOT_FOUND_SESSION":                 404,
		"NOT_FOUND_SUBSCRIPTION_HOST":       404,
		"NOT_FOUND_NOTIFICATION":            404,
		"NOT_FOUND_GEO_FILE":                404,
		"NOT_FOUND_AUDIT_LOG":               404,
		"NOT_FOUND_UPGRADE_TASK":            404,
		"NOT_FOUND_ENTRY_GROUP":             404,
		"CONFLICT_SUBSCRIPTION_HOST_EXISTS": 409,
		"CONFLICT_CAPABILITY_MISSING":       409,
		"CONFLICT_NODE_UPGRADING":           409,
		"CONFLICT_UPGRADE_TARGET_INVALID":   409,
		"CONFLICT_COMPOSE_MANUAL_REQUIRED":  409,
		"VALIDATION_INVALID_RANGE":          400,
		"VALIDATION_INVALID_SORT":           400,
		"VALIDATION_UNSUPPORTED_FORMAT":     400,
		"VALIDATION_INVALID_FILE_TYPE":      400,
		"VALIDATION_FILE_TOO_LARGE":         400,
		"VALIDATION_UPDATE_SOURCE_INVALID":  400,
		"GEO_PARSE_FAILED":                  400,
		"UPSTREAM_AGENT_UNAVAILABLE":        502,
		"UPSTREAM_NOTIFICATION_FAILED":      502,
		"UPSTREAM_UPDATE_SOURCE_FAILED":     502,
		"UPSTREAM_SIGNATURE_INVALID":        502,
		"UPSTREAM_UPGRADER_UNAVAILABLE":     503,
	}

	for code, status := range want {
		c, ok := byCode[code]
		if !ok {
			t.Errorf("error code %s is not registered", code)
			continue
		}
		if c.HTTP != status {
			t.Errorf("error code %s http = %d, want %d", code, c.HTTP, status)
		}
		if c.I18n != "errors."+code {
			t.Errorf("error code %s i18n = %q, want errors.%s", code, c.I18n, code)
		}
		if c.Message == "" || c.MessageEn == "" {
			t.Errorf("error code %s missing i18n messages", code)
		}
	}
}
