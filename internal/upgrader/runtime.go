package upgrader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// SystemctlRunner 通过 systemctl 重启受管服务（design.md §7.3）。
type SystemctlRunner struct{}

func (SystemctlRunner) Restart(ctx context.Context, service string) error {
	out, err := exec.CommandContext(ctx, "systemctl", "restart", service).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl restart %s: %s", service, string(out))
	}
	return nil
}

// SymlinkLinker 原子切换 <base>/current 软链。
type SymlinkLinker struct{}

func (SymlinkLinker) Current(baseDir string) (string, error) {
	current := filepath.Join(baseDir, "current")
	target, err := os.Readlink(current)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return target, nil
}

func (SymlinkLinker) Switch(baseDir, target string) error {
	current := filepath.Join(baseDir, "current")
	tmp := filepath.Join(baseDir, fmt.Sprintf(".current.tmp.%d", time.Now().UnixNano()))
	if err := os.Remove(tmp); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, current)
}

// WaitPanelHealth 轮询 /readyz 直到 200 或超时。
func WaitPanelHealth(ctx context.Context, url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 3 * time.Second}
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("readyz returned %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("health check timed out")
	}
	return lastErr
}
