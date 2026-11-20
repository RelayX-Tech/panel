package health

import (
	"encoding/json"
	"net/http"
)

type statusBody struct {
	Status string `json:"status"`
}

func writeStatus(w http.ResponseWriter, code int, status string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	body, _ := json.Marshal(statusBody{Status: status})
	_, _ = w.Write(body)
}

// Healthz 是存活探针：进程可响应即 200，不检查外部依赖。
func Healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeStatus(w, http.StatusOK, "ok")
	}
}

// Readyz 是就绪探针：所有依赖可用时返回 200，任一依赖不可用时返回 503，
// 不泄露内部错误细节。
func Readyz(checks ...func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		for _, check := range checks {
			if check == nil {
				continue
			}
			if err := check(); err != nil {
				writeStatus(w, http.StatusServiceUnavailable, "unavailable")
				return
			}
		}
		writeStatus(w, http.StatusOK, "ok")
	}
}
