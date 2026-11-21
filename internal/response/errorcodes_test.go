package response

import (
	"testing"

	"github.com/relayx-tech/panel/internal/errcode"
)

func TestResponseCodesAreRegistered(t *testing.T) {
	cases := []*AppError{
		InvalidCredentials(),
		Unauthorized(),
		SessionExpired(),
		CSRFRejected(),
		MissingField("account"),
		InvalidField("password", "too long"),
		RateLimited(30),
		Internal(nil),
	}
	for _, appErr := range cases {
		status, ok := errcode.HTTPStatus[appErr.Code]
		if !ok {
			t.Errorf("code %q is not registered", appErr.Code)
			continue
		}
		if status != appErr.Status {
			t.Errorf("code %q status = %d, registry = %d", appErr.Code, appErr.Status, status)
		}
		if got, want := errcode.I18nKey[appErr.Code], "errors."+appErr.Code; got != want {
			t.Errorf("code %q i18n = %q, want %q", appErr.Code, got, want)
		}
	}

	if _, ok := errcode.HTTPStatus["OK"]; !ok {
		t.Error("OK is not registered")
	}
}
