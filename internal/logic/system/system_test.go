package system_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/relayx-tech/panel/internal/config"
	systemlogic "github.com/relayx-tech/panel/internal/logic/system"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	panelsystem "github.com/relayx-tech/panel/internal/system"
)

type fakeNodeModel struct {
	model.NodeModel
	versions model.ComponentVersions
	err      error
}

func (f fakeNodeModel) ComponentVersions(context.Context) (model.ComponentVersions, error) {
	return f.versions, f.err
}

type fakeConn struct {
	sqlx.SqlConn
	err error
}

func (f fakeConn) QueryRowCtx(_ context.Context, v any, _ string, _ ...any) error {
	if f.err != nil {
		return f.err
	}
	if p, ok := v.(*int); ok {
		*p = 1
	}
	return nil
}

type fakePinger struct{ up bool }

func (f fakePinger) Ping() bool { return f.up }

func appErrCode(t *testing.T, err error) string {
	t.Helper()
	var appErr *response.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error %v is not *response.AppError", err)
	}
	return appErr.Code
}

func TestSystemInfoMapsComponentVersions(t *testing.T) {
	svcCtx := &svc.ServiceContext{NodeModel: fakeNodeModel{versions: model.ComponentVersions{
		AgentMin: "1.3.8", AgentMax: "1.4.2", Xray: "25.10.1",
	}}}
	resp, err := systemlogic.NewSystemInfoLogic(context.Background(), svcCtx).SystemInfo()
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	data := resp.Data
	if data.Panel.Version != panelsystem.Version || data.Panel.UptimeSeconds < 0 {
		t.Fatalf("unexpected panel info: %+v", data.Panel)
	}
	components := data.Components
	if components.AgentMin != "1.3.8" || components.AgentMax != "1.4.2" || components.Xray != "25.10.1" {
		t.Fatalf("unexpected agent/xray: %+v", components)
	}
	if components.CoreControl != "—" || components.CoreNodeMin != "—" || components.CoreNodeMax != "—" {
		t.Fatalf("empty platform versions should render dash: %+v", components)
	}
	if components.Contract != panelsystem.ContractVersion || components.Schema != panelsystem.SchemaVersion {
		t.Fatalf("unexpected contract/schema: %+v", components)
	}
	compat := data.Compatibility
	if !compat.Compatible || compat.ContractMajor != panelsystem.ContractMajor ||
		compat.SchemaVersion != panelsystem.SchemaVersion ||
		compat.AgentMinVersion != "1.3.8" || compat.AgentMaxVersion != "1.4.2" {
		t.Fatalf("unexpected compatibility: %+v", compat)
	}
}

func TestSystemInfoFallsBackToDash(t *testing.T) {
	svcCtx := &svc.ServiceContext{NodeModel: fakeNodeModel{}}
	resp, err := systemlogic.NewSystemInfoLogic(context.Background(), svcCtx).SystemInfo()
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if resp.Data.Components.AgentMin != "—" || resp.Data.Components.AgentMax != "—" ||
		resp.Data.Components.Xray != "—" {
		t.Fatalf("empty versions should render dash: %+v", resp.Data.Components)
	}
}

func TestSystemInfoWrapsModelError(t *testing.T) {
	svcCtx := &svc.ServiceContext{NodeModel: fakeNodeModel{err: errors.New("db down")}}
	_, err := systemlogic.NewSystemInfoLogic(context.Background(), svcCtx).SystemInfo()
	if code := appErrCode(t, err); code != "INTERNAL" {
		t.Fatalf("code = %q, want INTERNAL", code)
	}
}

func TestSystemHealthReportsAC85Items(t *testing.T) {
	svcCtx := &svc.ServiceContext{DB: fakeConn{}, Redis: fakePinger{up: true}}
	resp, err := systemlogic.NewSystemHealthLogic(context.Background(), svcCtx).SystemHealth()
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	statuses := map[string]string{}
	for _, item := range resp.Data.Items {
		statuses[item.Key] = item.Status
	}
	want := map[string]string{
		"healthz":      "ok",
		"readyz":       "ok",
		"database":     "ok",
		"redis":        "ok",
		"core-control": "disabled",
		"registration": "disabled",
		"agent":        "disabled",
	}
	for key, status := range want {
		if statuses[key] != status {
			t.Fatalf("%s status = %q, want %q (all: %+v)", key, statuses[key], status, resp.Data.Items)
		}
	}
	if len(resp.Data.Items) != len(want) {
		t.Fatalf("items = %d, want %d", len(resp.Data.Items), len(want))
	}
	if _, err := time.Parse(time.RFC3339, resp.Data.CheckedAt); err != nil {
		t.Fatalf("checkedAt %q is not RFC3339: %v", resp.Data.CheckedAt, err)
	}
}

func TestSystemHealthDatabaseErrorDegradesReady(t *testing.T) {
	svcCtx := &svc.ServiceContext{DB: fakeConn{err: errors.New("db down")}, Redis: fakePinger{up: true}}
	resp, err := systemlogic.NewSystemHealthLogic(context.Background(), svcCtx).SystemHealth()
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	statuses := map[string]string{}
	for _, item := range resp.Data.Items {
		statuses[item.Key] = item.Status
	}
	if statuses["database"] != "error" || statuses["readyz"] != "degraded" {
		t.Fatalf("unexpected statuses: %+v", statuses)
	}
}

func TestSystemHealthRedisDownDegradesReady(t *testing.T) {
	svcCtx := &svc.ServiceContext{DB: fakeConn{}, Redis: fakePinger{up: false}}
	resp, err := systemlogic.NewSystemHealthLogic(context.Background(), svcCtx).SystemHealth()
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	statuses := map[string]string{}
	for _, item := range resp.Data.Items {
		statuses[item.Key] = item.Status
	}
	if statuses["redis"] != "error" || statuses["readyz"] != "degraded" {
		t.Fatalf("unexpected statuses: %+v", statuses)
	}
}

func TestSystemHealthRegistrationEnabled(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		DB:     fakeConn{},
		Redis:  fakePinger{up: true},
		Config: config.Config{Registration: config.RegistrationConf{Enabled: true}},
	}
	resp, err := systemlogic.NewSystemHealthLogic(context.Background(), svcCtx).SystemHealth()
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	statuses := map[string]string{}
	for _, item := range resp.Data.Items {
		statuses[item.Key] = item.Status
	}
	if statuses["registration"] != "ok" {
		t.Fatalf("registration = %q, want ok", statuses["registration"])
	}
}
