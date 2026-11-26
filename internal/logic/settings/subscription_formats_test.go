package settings_test

import (
	"context"
	"testing"

	"github.com/relayx-tech/panel/internal/config"
	"github.com/relayx-tech/panel/internal/logic/settings"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

func TestSubscriptionFormatsCapabilityConflict(t *testing.T) {
	// DB 有 v2ray、clash；能力只有 v2ray、singbox。
	formats := &fakeSubscriptionModel{formats: []model.SubscriptionFormat{
		{Key: "v2ray", Label: "V2Ray", Enabled: true, SortOrder: 1},
		{Key: "clash", Label: "Clash", Enabled: true, SortOrder: 2},
	}}
	cfg := config.Config{}
	cfg.Subscription.Capabilities = []string{"v2ray", "singbox"}
	svcCtx := &svc.ServiceContext{SubscriptionModel: formats, Config: cfg}

	resp, err := settings.NewSubscriptionFormatsLogic(context.Background(), svcCtx).SubscriptionFormats()
	if err != nil {
		t.Fatalf("formats: %v", err)
	}
	byKey := map[string]types.SubscriptionFormat{}
	for _, f := range resp.Data.Formats {
		byKey[f.Key] = f
	}
	if byKey["clash"].Enabled || byKey["clash"].CapabilitySupported {
		t.Fatalf("clash should be disabled and unsupported: %+v", byKey["clash"])
	}
	if !byKey["v2ray"].Enabled || !byKey["v2ray"].CapabilitySupported {
		t.Fatalf("v2ray should stay enabled: %+v", byKey["v2ray"])
	}
	if f, ok := byKey["singbox"]; !ok || f.Enabled || f.Source != "capability" {
		t.Fatalf("singbox should be read-only fallback: %+v", byKey["singbox"])
	}
}
