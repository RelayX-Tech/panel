package upgrader

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Server 暴露 UDS HTTP/JSON 协议（design.md §7.3）。
type Server struct {
	Applier Applier
	Store   Store
	Token   string

	mu      sync.Mutex
	running bool
	current string
	history []State
	maxKeep int
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.auth(s.handleStatus))
	mux.HandleFunc("/upgrade", s.auth(s.handleUpgrade))
	mux.HandleFunc("/rollback", s.auth(s.handleRollback))
	mux.HandleFunc("/history", s.auth(s.handleHistory))
	mux.HandleFunc("/healthz", s.auth(s.handleHealthz))
	return mux
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Relayx-Upgrader-Token")
		if s.Token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.Token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	state, err := s.Store.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	s.mu.Lock()
	items := append([]State(nil), s.history...)
	s.mu.Unlock()
	if len(items) > limit {
		items = items[len(items)-limit:]
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.OperationID == "" || req.TargetVersion == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "operationId and targetVersion are required"})
		return
	}
	if !s.begin(req.OperationID) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "another operation is in progress"})
		return
	}
	go s.run(req)
	writeJSON(w, http.StatusOK, map[string]any{"accepted": true, "state": "running"})
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.OperationID == "" {
		req.OperationID = "rollback-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	if !s.begin(req.OperationID) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "another operation is in progress"})
		return
	}
	go func(req Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_ = s.Applier.Rollback(ctx, req)
		s.finish()
	}(req)
	writeJSON(w, http.StatusOK, map[string]any{"accepted": true, "state": "rolling_back"})
}

// begin 单飞：同 operationId 幂等返回 false 的调用方应读 /status。
func (s *Server) begin(operationID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return false
	}
	s.running = true
	s.current = operationID
	return true
}

func (s *Server) run(req Request) {
	timeout := 30 * time.Minute
	if req.DeadlineSeconds > 0 {
		timeout = time.Duration(req.DeadlineSeconds) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_ = s.Applier.Apply(ctx, req)
	s.finish()
}

func (s *Server) finish() {
	state, _ := s.Store.Load()
	s.mu.Lock()
	s.running = false
	s.current = ""
	s.history = append(s.history, state)
	keep := s.maxKeep
	if keep <= 0 {
		keep = 50
	}
	if len(s.history) > keep {
		s.history = s.history[len(s.history)-keep:]
	}
	s.mu.Unlock()
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
