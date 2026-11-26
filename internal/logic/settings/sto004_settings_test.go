package settings_test

import (
	"context"
	"errors"
	"testing"

	"github.com/relayx-tech/panel/internal/cryptobox"
	"github.com/relayx-tech/panel/internal/logic/settings"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type fakeSubscriptionModel struct {
	model.SubscriptionModel
	hosts   []model.SubscriptionHost
	formats []model.SubscriptionFormat
}

func (f *fakeSubscriptionModel) ListHosts(context.Context, int, int) ([]model.SubscriptionHost, int64, error) {
	return f.hosts, int64(len(f.hosts)), nil
}
func (f *fakeSubscriptionModel) CreateHost(_ context.Context, h model.SubscriptionHost) (*model.SubscriptionHost, error) {
	h.ID = 1
	h.TLSStatus = "pending"
	f.hosts = append(f.hosts, h)
	return &h, nil
}
func (f *fakeSubscriptionModel) ListFormats(context.Context) ([]model.SubscriptionFormat, error) {
	return f.formats, nil
}

type fakeNotificationModel struct {
	model.NotificationModel
	created *model.NotificationChannel
}

func (f *fakeNotificationModel) Create(_ context.Context, c model.NotificationChannel) (*model.NotificationChannel, error) {
	c.ID = 1
	f.created = &c
	return &c, nil
}

func settingsErrCode(err error) string {
	var appErr *response.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func TestSubscriptionHostValidation(t *testing.T) {
	svcCtx := &svc.ServiceContext{SubscriptionModel: &fakeSubscriptionModel{}}
	l := settings.NewSubscriptionHostCreateLogic(context.Background(), svcCtx)

	if _, err := l.SubscriptionHostCreate(&types.SubscriptionHostCreateReq{}); err == nil {
		t.Fatal("empty host should fail")
	}
	if _, err := l.SubscriptionHostCreate(&types.SubscriptionHostCreateReq{Host: "bad host", EntryGroupId: "eg1"}); settingsErrCode(err) != "VALIDATION_INVALID_FIELD" {
		t.Fatalf("code = %q", settingsErrCode(err))
	}
	resp, err := l.SubscriptionHostCreate(&types.SubscriptionHostCreateReq{Host: "Sub.Example.com", EntryGroupId: "eg1", TlsMode: "panel"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if resp.Data.Item.Host != "sub.example.com" || resp.Data.Item.TlsStatus == "" {
		t.Fatalf("unexpected host: %+v", resp.Data.Item)
	}
}

func TestSubscriptionPreviewUnsupportedFormat(t *testing.T) {
	svcCtx := &svc.ServiceContext{SubscriptionModel: &fakeSubscriptionModel{
		formats: []model.SubscriptionFormat{{Key: "v2ray", Label: "V2Ray", Enabled: true, CapabilitySupported: true}},
	}}
	l := settings.NewSubscriptionPreviewLogic(context.Background(), svcCtx)
	if _, err := l.SubscriptionPreview(&types.SubscriptionPreviewReq{EntryGroupId: "eg1", Format: "bogus"}); settingsErrCode(err) != "VALIDATION_UNSUPPORTED_FORMAT" {
		t.Fatalf("code = %q", settingsErrCode(err))
	}
	resp, err := l.SubscriptionPreview(&types.SubscriptionPreviewReq{EntryGroupId: "eg1", Format: "v2ray"})
	if err != nil || resp.Data.Url == "" {
		t.Fatalf("preview: %+v err=%v", resp, err)
	}
}

func TestNotificationCreateEncryptsTarget(t *testing.T) {
	key := make([]byte, 32)
	box, err := cryptobox.New(key, "")
	if err != nil {
		t.Fatal(err)
	}
	notifications := &fakeNotificationModel{}
	svcCtx := &svc.ServiceContext{NotificationModel: notifications, Box: box}
	l := settings.NewNotificationCreateLogic(context.Background(), svcCtx)

	if _, err := l.NotificationCreate(&types.NotificationCreateReq{Name: "ops", ChannelType: "webhook", Target: "https://example.com/hook", MinLevel: "warning", Enabled: true}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if notifications.created == nil || string(notifications.created.TargetEncrypted) == "https://example.com/hook" {
		t.Fatal("target must be encrypted at rest")
	}
	if notifications.created.TargetMasked != "••••••" {
		t.Fatalf("mask = %q", notifications.created.TargetMasked)
	}
}

func TestNotificationCreateRequiresBox(t *testing.T) {
	l := settings.NewNotificationCreateLogic(context.Background(), &svc.ServiceContext{NotificationModel: &fakeNotificationModel{}})
	_, err := l.NotificationCreate(&types.NotificationCreateReq{Name: "ops", ChannelType: "webhook", Target: "https://example.com/hook", MinLevel: "warning"})
	if err == nil {
		t.Fatal("missing master key should fail")
	}
}
