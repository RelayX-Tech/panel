// Package upgrader 实现 relayx-upgrader：Panel 自升级的外挂执行器（design.md §7.3）。
package upgrader

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// State 是持久化到 state.json 的运行状态。
type State struct {
	State       string      `json:"state"`
	OperationID string      `json:"operationId"`
	FromVersion string      `json:"fromVersion"`
	ToVersion   string      `json:"toVersion"`
	Phase       string      `json:"phase"`
	Progress    int         `json:"progress"`
	StartedAt   string      `json:"startedAt"`
	FinishedAt  string      `json:"finishedAt"`
	Error       *StateError `json:"error,omitempty"`
}

type StateError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Store 读写状态文件。
type Store interface {
	Load() (State, error)
	Save(State) error
}

// FileStore 以 state.json 持久化（原子写：临时文件 + rename）。
type FileStore struct {
	Path string
	mu   sync.Mutex
}

func (s *FileStore) Load() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{State: "idle"}, nil
		}
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{State: "idle"}, nil
	}
	return state, nil
}

func (s *FileStore) Save(state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o750); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }
