package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/relayx-tech/panel/internal/config"
)

const swaggerArtifact = "../../docs/openapi/panel.swagger.json"

func projectRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}
	return abs
}

func goctl(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("goctl")
	if err != nil {
		t.Skip("goctl not installed; skipping contract test")
	}
	return path
}

func TestAPIContractValidates(t *testing.T) {
	bin := goctl(t)
	root := projectRoot(t)
	cmd := exec.Command(bin, "api", "validate", "-api", filepath.Join(root, "api", "panel.api"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("goctl api validate: %v\n%s", err, out)
	}
}

func TestSwaggerArtifactMatchesContract(t *testing.T) {
	bin := goctl(t)
	root := projectRoot(t)
	tmp := t.TempDir()

	cmd := exec.Command(bin, "api", "swagger",
		"--api", filepath.Join(root, "api", "panel.api"),
		"--dir", tmp, "--filename", "panel")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("goctl api swagger: %v\n%s", err, out)
	}

	generated := readJSON(t, filepath.Join(tmp, "panel.json"))
	committed := readJSON(t, filepath.Join(root, "docs", "openapi", "panel.swagger.json"))
	// goctl 每次生成都会写入 x-date 时间戳，比较前移除以保证契约可重复校验。
	delete(generated, "x-date")
	delete(committed, "x-date")
	if !reflect.DeepEqual(generated, committed) {
		t.Fatal("docs/openapi/panel.swagger.json is stale; regenerate with:\n" +
			"  goctl api swagger --api api/panel.api --dir docs/openapi --filename panel\n" +
			"  then remove the top-level \"x-date\" key")
	}

	paths, _ := committed["paths"].(map[string]any)
	for _, p := range []string{"/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/auth/session"} {
		if _, ok := paths[p]; !ok {
			t.Errorf("swagger missing path %s", p)
		}
	}
}

func TestSwaggerEnvelopeFieldsPresent(t *testing.T) {
	root := projectRoot(t)
	spec := readJSON(t, filepath.Join(root, "docs", "openapi", "panel.swagger.json"))
	paths := spec["paths"].(map[string]any)

	for _, tc := range []struct{ path, method string }{
		{"/api/v1/auth/login", "post"},
		{"/api/v1/auth/logout", "post"},
		{"/api/v1/auth/session", "get"},
	} {
		op := paths[tc.path].(map[string]any)[tc.method].(map[string]any)
		schema := op["responses"].(map[string]any)["200"].(map[string]any)["schema"].(map[string]any)
		props := schema["properties"].(map[string]any)
		for _, field := range []string{"code", "message", "data", "requestId"} {
			if _, ok := props[field]; !ok {
				t.Errorf("%s %s response missing %q", tc.method, tc.path, field)
			}
		}
	}
}

func TestSessionCookieNameContract(t *testing.T) {
	c, err := config.Load(filepath.Join(projectRoot(t), "etc", "panel-api.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if c.Auth.SessionCookieName != "relayx_session" {
		t.Errorf("cookie name = %q, want relayx_session (frontend config.ts must match)", c.Auth.SessionCookieName)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return out
}
