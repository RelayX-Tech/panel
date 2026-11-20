package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zeromicro/go-zero/rest/httpx"

	"github.com/relayx-tech/panel/internal/requestid"
)

type userPayload struct {
	ID string `json:"id"`
}

func TestOKWritesSuccessEnvelope(t *testing.T) {
	InstallErrorHandler()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(requestid.NewContext(req.Context(), "01J8Z6YQ9K3N4R5T6V7W8X9Y0Z"))
	rec := httptest.NewRecorder()

	OK(rec, req, userPayload{ID: "u-1"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if got["code"] != "OK" || got["message"] != "" || got["requestId"] != "01J8Z6YQ9K3N4R5T6V7W8X9Y0Z" {
		t.Errorf("envelope = %#v", got)
	}
	data, ok := got["data"].(map[string]any)
	if !ok || data["id"] != "u-1" {
		t.Errorf("data = %#v", got["data"])
	}
}

func TestOKWritesNullData(t *testing.T) {
	InstallErrorHandler()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	OK(rec, req, nil)

	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["data"] != nil {
		t.Errorf("data = %#v, want null", got["data"])
	}
}

func TestErrorHandlerMapsAppError(t *testing.T) {
	InstallErrorHandler()

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = req.WithContext(requestid.NewContext(req.Context(), "01J8Z6YQ9K3N4R5T6V7W8X9Y0Z"))
	rec := httptest.NewRecorder()

	appErr := &AppError{
		Status:  http.StatusBadRequest,
		Code:    "VALIDATION_MISSING_FIELD",
		Message: "required field missing",
		Field:   "password",
		Details: []FieldError{{Field: "password", Code: "REQUIRED", Message: "password is required"}},
	}
	httpx.ErrorCtx(req.Context(), rec, appErr)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if got["code"] != "VALIDATION_MISSING_FIELD" || got["field"] != "password" {
		t.Errorf("envelope = %#v", got)
	}
	if got["data"] != nil {
		t.Errorf("data = %#v, want null", got["data"])
	}
	if _, ok := got["details"].([]any); !ok {
		t.Errorf("details missing: %#v", got["details"])
	}
	if got["requestId"] != "01J8Z6YQ9K3N4R5T6V7W8X9Y0Z" {
		t.Errorf("requestId = %#v", got["requestId"])
	}
}

func TestErrorHandlerHidesInternalErrors(t *testing.T) {
	InstallErrorHandler()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	httpx.ErrorCtx(req.Context(), rec, errors.New("pq: password authentication failed for user relayx"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	for _, leaked := range []string{"pq:", "password", "relayx"} {
		if strings.Contains(body, leaked) {
			t.Errorf("internal error leaked %q in %s", leaked, body)
		}
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["code"] != "INTERNAL" {
		t.Errorf("code = %#v", got["code"])
	}
}
