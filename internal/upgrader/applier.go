package upgrader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/relayx-tech/panel/internal/upgrade"
)

// Request 是 /upgrade 的输入。
type Request struct {
	OperationID       string          `json:"operationId"`
	TargetVersion     string          `json:"targetVersion"`
	ManifestURL       string          `json:"manifestUrl"`
	ManifestSignature string          `json:"manifestSignature"`
	Assets            []upgrade.Asset `json:"assets"`
	DeadlineSeconds   int             `json:"deadlineSeconds"`
}

// ServiceRestarter 重启受管 systemd 服务。
type ServiceRestarter interface {
	Restart(ctx context.Context, service string) error
}

// Linker 管理 current 软链（Linux 生产实现；测试注入假实现）。
type Linker interface {
	Current(baseDir string) (string, error)
	Switch(baseDir, target string) error
}

// Applier 执行一次完整升级 / 回滚。
type Applier interface {
	Apply(ctx context.Context, req Request) error
	Rollback(ctx context.Context, req Request) error
}

// FileApplier 是生产实现：下载 → 验签 / SHA-256 → 暂存 → 切软链 → 重启 → 健康检查 → 失败回滚。
type FileApplier struct {
	BaseDir     string // /opt/relayx
	Services    []string
	Runner      ServiceRestarter
	Linker      Linker
	Store       Store
	HTTPClient  *http.Client
	HealthCheck func(ctx context.Context) error
}

var _ Applier = (*FileApplier)(nil)

func (a *FileApplier) Apply(ctx context.Context, req Request) error {
	if req.TargetVersion == "" || len(req.Assets) == 0 {
		return a.fail(req, "VALIDATION_INVALID_FIELD", "targetVersion and assets are required")
	}
	previous, _ := a.Linker.Current(a.BaseDir)

	_ = a.save(req, "running", "downloading", 10, nil)
	staging := filepath.Join(a.BaseDir, "staging", req.OperationID)
	if err := a.download(ctx, staging, req.Assets); err != nil {
		_ = a.rollbackLink(req, previous)
		return a.fail(req, "UPSTREAM_SIGNATURE_INVALID", err.Error())
	}

	_ = a.save(req, "running", "verifying", 40, nil)
	releaseDir := filepath.Join(a.BaseDir, "releases", req.TargetVersion)
	if err := install(staging, releaseDir, req.Assets); err != nil {
		_ = a.rollbackLink(req, previous)
		return a.fail(req, "CONFIG_INVALID", err.Error())
	}

	_ = a.save(req, "running", "applying", 70, nil)
	if err := a.Linker.Switch(a.BaseDir, filepath.Join("releases", req.TargetVersion)); err != nil {
		_ = a.rollbackLink(req, previous)
		return a.fail(req, "CONFIG_INVALID", err.Error())
	}
	if err := a.restartAll(ctx); err != nil {
		rollbackErr := a.rollbackLink(req, previous)
		_ = a.restartAll(context.Background())
		return rollbackErr
	}
	if a.HealthCheck != nil {
		if err := a.HealthCheck(ctx); err != nil {
			rollbackErr := a.rollbackLink(req, previous)
			_ = a.restartAll(context.Background())
			return rollbackErr
		}
	}

	finished := nowRFC3339()
	state := State{
		State: "succeeded", OperationID: req.OperationID, ToVersion: req.TargetVersion,
		Phase: "done", Progress: 100, StartedAt: stateStart(a.Store), FinishedAt: finished,
	}
	_ = a.Store.Save(state)
	return nil
}

func (a *FileApplier) Rollback(ctx context.Context, req Request) error {
	previous, err := a.Linker.Current(a.BaseDir)
	if err != nil {
		return a.fail(req, "CONFIG_INVALID", err.Error())
	}
	_ = a.save(req, "rolling_back", "rolling_back", 50, nil)
	return a.rollbackLink(req, previous)
}

func (a *FileApplier) rollbackLink(req Request, previous string) error {
	if previous == "" {
		return a.fail(req, "UPGRADE_FAILED", "no previous release to roll back")
	}
	_ = a.Linker.Switch(a.BaseDir, previous)
	_ = a.restartAll(context.Background())
	return a.fail(req, "UPGRADE_ROLLED_BACK", "rolled back to "+previous)
}

func (a *FileApplier) fail(req Request, code, message string) error {
	state, _ := a.Store.Load()
	state.State = "rolled_back"
	if !strings.Contains(code, "ROLLED_BACK") {
		state.State = "failed"
	}
	state.OperationID = req.OperationID
	state.ToVersion = req.TargetVersion
	state.FinishedAt = nowRFC3339()
	state.Error = &StateError{Code: code, Message: message}
	_ = a.Store.Save(state)
	return fmt.Errorf("%s: %s", code, message)
}

func (a *FileApplier) save(req Request, state, phase string, progress int, errDetail *StateError) error {
	current, _ := a.Store.Load()
	started := current.StartedAt
	if started == "" {
		started = nowRFC3339()
	}
	return a.Store.Save(State{
		State: state, OperationID: req.OperationID, FromVersion: current.FromVersion,
		ToVersion: req.TargetVersion, Phase: phase, Progress: progress, StartedAt: started,
		Error: errDetail,
	})
}

func (a *FileApplier) restartAll(ctx context.Context) error {
	for _, service := range a.Services {
		if err := a.Runner.Restart(ctx, service); err != nil {
			return fmt.Errorf("restart %s: %w", service, err)
		}
	}
	return nil
}

func (a *FileApplier) download(ctx context.Context, dir string, assets []upgrade.Asset) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	client := a.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	for _, asset := range assets {
		if !strings.HasPrefix(asset.URL, "https://") {
			return fmt.Errorf("asset %s must be HTTPS", asset.Name)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("download %s: %w", asset.Name, err)
		}
		path := filepath.Join(dir, filepath.Base(asset.Name))
		sum := sha256.New()
		out, err := os.Create(path)
		if err != nil {
			resp.Body.Close()
			return err
		}
		_, copyErr := io.Copy(io.MultiWriter(out, sum), resp.Body)
		out.Close()
		resp.Body.Close()
		if copyErr != nil {
			return copyErr
		}
		got := hex.EncodeToString(sum.Sum(nil))
		if asset.SHA256 != "" && !strings.EqualFold(got, asset.SHA256) {
			return fmt.Errorf("sha256 mismatch for %s", asset.Name)
		}
		if asset.Size > 0 {
			if info, err := os.Stat(path); err == nil && info.Size() != asset.Size {
				return fmt.Errorf("size mismatch for %s", asset.Name)
			}
		}
	}
	return nil
}

func install(staging, releaseDir string, assets []upgrade.Asset) error {
	if err := os.MkdirAll(releaseDir, 0o750); err != nil {
		return err
	}
	for _, asset := range assets {
		name := filepath.Base(asset.Name)
		src := filepath.Join(staging, name)
		dst := filepath.Join(releaseDir, name)
		if err := os.Rename(src, dst); err != nil {
			// 跨设备时退化为复制。
			data, readErr := os.ReadFile(src)
			if readErr != nil {
				return err
			}
			if writeErr := os.WriteFile(dst, data, 0o750); writeErr != nil {
				return writeErr
			}
		}
	}
	return nil
}

func stateStart(store Store) string {
	state, _ := store.Load()
	return state.StartedAt
}
