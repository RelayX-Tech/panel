package settings_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/config"
	"github.com/relayx-tech/panel/internal/logic/settings"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type fakeSettingModel struct {
	model.SettingModel
	values map[string][]byte
}

func (f *fakeSettingModel) GetJSON(_ context.Context, key string) ([]byte, bool, error) {
	v, ok := f.values[key]
	return v, ok, nil
}
func (f *fakeSettingModel) SetJSON(_ context.Context, key string, value []byte) error {
	f.values[key] = value
	return nil
}

type fakeApplyModel struct {
	model.SettingsApplyModel
	task *model.SettingsApplyTask
}

func (f fakeApplyModel) LatestTask(context.Context, string) (*model.SettingsApplyTask, error) {
	return f.task, nil
}
func (f fakeApplyModel) CreateTask(context.Context, *model.SettingsApplyTask) error { return nil }
func (f fakeApplyModel) UpsertNode(context.Context, *model.SettingsApplyNode) error { return nil }

func errCode(err error) string {
	var appErr *response.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func testConfig() config.Config {
	c := config.Config{}
	c.Host = "0.0.0.0"
	c.Port = 8888
	c.Auth = config.AuthConf{
		SessionCookieName:       "relayx_session",
		SessionIdleTTL:          24 * time.Hour,
		SessionAbsoluteTTL:      168 * time.Hour,
		CookieSecure:            true,
		AllowedOrigins:          []string{"https://panel.example.com"},
		LoginRateLimit:          config.LoginRateLimitConf{Window: time.Minute, MaxAttempts: 10},
		PasswordChangeRateLimit: config.PasswordChangeRateLimitConf{Window: 15 * time.Minute, MaxAttempts: 5},
		TrustedProxyCIDRs:       []string{"10.0.0.0/8"},
	}
	return c
}

func TestGeneralSettings(t *testing.T) {
	l := settings.NewGeneralSettingsLogic(context.Background(), &svc.ServiceContext{Config: testConfig()})
	resp, err := l.GeneralSettings()
	if err != nil {
		t.Fatalf("general: %v", err)
	}
	if resp.Data.Listen != "0.0.0.0:8888" {
		t.Fatalf("listen = %q", resp.Data.Listen)
	}
	if resp.Data.SessionIdleTtlSeconds != 86400 || resp.Data.SessionAbsoluteTtlSeconds != 604800 {
		t.Fatalf("ttl = %d/%d", resp.Data.SessionIdleTtlSeconds, resp.Data.SessionAbsoluteTtlSeconds)
	}
	if resp.Data.DeploymentMode != "binary" {
		t.Fatalf("deploymentMode = %q", resp.Data.DeploymentMode)
	}
}

func TestSecuritySettings(t *testing.T) {
	l := settings.NewSecuritySettingsLogic(context.Background(), &svc.ServiceContext{Config: testConfig()})
	resp, err := l.SecuritySettings()
	if err != nil {
		t.Fatalf("security: %v", err)
	}
	if !resp.Data.CookieSecure || len(resp.Data.AllowedOrigins) != 1 || resp.Data.LoginRateLimit.MaxAttempts != 10 {
		t.Fatalf("unexpected security: %+v", resp.Data)
	}
}

func TestProbeSettingsDefaults(t *testing.T) {
	store := &fakeSettingModel{values: map[string][]byte{}}
	l := settings.NewProbeSettingsLogic(context.Background(), &svc.ServiceContext{SettingModel: store})
	resp, err := l.ProbeSettings()
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if resp.Data.FrequencySeconds != 30 || resp.Data.TimeoutMs != 5000 || resp.Data.FailureThreshold != 3 || resp.Data.RecoveryThreshold != 3 {
		t.Fatalf("unexpected defaults: %+v", resp.Data)
	}
}

func TestProbeSettingsUpdateValidation(t *testing.T) {
	store := &fakeSettingModel{values: map[string][]byte{}}
	svcCtx := &svc.ServiceContext{SettingModel: store, SettingsApplyModel: fakeApplyModel{}}
	l := settings.NewProbeSettingsUpdateLogic(context.Background(), svcCtx)

	bad := []*types.ProbeSettingsUpdateReq{
		{FrequencySeconds: 0, TimeoutMs: 5000, FailureThreshold: 3, RecoveryThreshold: 3},
		{FrequencySeconds: 30, TimeoutMs: 0, FailureThreshold: 3, RecoveryThreshold: 3},
		{FrequencySeconds: 30, TimeoutMs: 5000, FailureThreshold: 0, RecoveryThreshold: 3},
		{FrequencySeconds: 30, TimeoutMs: 5000, FailureThreshold: 3, RecoveryThreshold: 11},
	}
	for _, req := range bad {
		if _, err := l.ProbeSettingsUpdate(req); err == nil {
			t.Fatalf("request %+v should be rejected", req)
		}
	}

	good := &types.ProbeSettingsUpdateReq{FrequencySeconds: 60, TimeoutMs: 8000, FailureThreshold: 4, RecoveryThreshold: 2}
	resp, err := l.ProbeSettingsUpdate(good)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if resp.Data.FrequencySeconds != 60 || store.values["probe"] == nil {
		t.Fatalf("not persisted: %+v", resp.Data)
	}
}

func TestProbeApplyStatus(t *testing.T) {
	now := time.Now().UTC()
	task := &model.SettingsApplyTask{
		ID: "t1", Kind: "probe", State: "running", UpdatedAt: now,
		Nodes: []model.SettingsApplyNode{{NodeID: "n1", NodeName: "node-1", State: "pending", Version: 2, ElapsedMs: 10}},
	}
	l := settings.NewProbeApplyStatusLogic(context.Background(), &svc.ServiceContext{SettingsApplyModel: fakeApplyModel{task: task}})
	resp, err := l.ProbeApplyStatus()
	if err != nil {
		t.Fatalf("apply status: %v", err)
	}
	if resp.Data.TaskId != "t1" || len(resp.Data.Nodes) != 1 || resp.Data.Nodes[0].NodeName != "node-1" {
		t.Fatalf("unexpected status: %+v", resp.Data)
	}
}
