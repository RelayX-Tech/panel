package settings_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/relayx-tech/panel/internal/config"
	"github.com/relayx-tech/panel/internal/logic/settings"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

// ---- fakes ----

type geoModelFunc struct {
	model.GeoModel
	getFn               func(context.Context, int64) (*model.GeoFile, error)
	upsertFn            func(context.Context, model.GeoFile) (*model.GeoFile, error)
	replaceCategoriesFn func(context.Context, int64, []model.GeoCategory) error
}

func (f geoModelFunc) Get(ctx context.Context, id int64) (*model.GeoFile, error) {
	if f.getFn != nil {
		return f.getFn(ctx, id)
	}
	return nil, model.ErrNotFound
}
func (f geoModelFunc) Upsert(ctx context.Context, file model.GeoFile) (*model.GeoFile, error) {
	if f.upsertFn != nil {
		return f.upsertFn(ctx, file)
	}
	return &model.GeoFile{ID: 1, Kind: file.Kind, Name: file.Name}, nil
}
func (f geoModelFunc) ReplaceCategories(ctx context.Context, fileID int64, cats []model.GeoCategory) error {
	if f.replaceCategoriesFn != nil {
		return f.replaceCategoriesFn(ctx, fileID, cats)
	}
	return nil
}

type notificationModelFunc struct {
	model.NotificationModel
	getFn    func(context.Context, int64) (*model.NotificationChannel, error)
	updateFn func(context.Context, int64, model.NotificationPatch) (*model.NotificationChannel, error)
}

func (f notificationModelFunc) Get(ctx context.Context, id int64) (*model.NotificationChannel, error) {
	if f.getFn != nil {
		return f.getFn(ctx, id)
	}
	return nil, model.ErrNotFound
}
func (f notificationModelFunc) Update(ctx context.Context, id int64, p model.NotificationPatch) (*model.NotificationChannel, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, id, p)
	}
	return nil, model.ErrNotFound
}

func writeGeoFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write geo file: %v", err)
	}
	return path
}

// ---- Geo 条目 ----

func TestGeoEntryListRejectsInvalidID(t *testing.T) {
	l := settings.NewGeoEntryListLogic(context.Background(), &svc.ServiceContext{GeoModel: geoModelFunc{}})
	for _, id := range []string{"abc", "0", "-3"} {
		if _, err := l.GeoEntryList(&types.GeoEntryListReq{Id: id}); errCode(err) != "NOT_FOUND_GEO_FILE" {
			t.Fatalf("id %q code = %q, want NOT_FOUND_GEO_FILE", id, errCode(err))
		}
	}
}

func TestGeoEntryListNotFoundAndInternal(t *testing.T) {
	l := settings.NewGeoEntryListLogic(context.Background(), &svc.ServiceContext{GeoModel: geoModelFunc{
		getFn: func(context.Context, int64) (*model.GeoFile, error) { return nil, model.ErrNotFound },
	}})
	if _, err := l.GeoEntryList(&types.GeoEntryListReq{Id: "1"}); errCode(err) != "NOT_FOUND_GEO_FILE" {
		t.Fatalf("not-found code = %q", errCode(err))
	}

	l = settings.NewGeoEntryListLogic(context.Background(), &svc.ServiceContext{GeoModel: geoModelFunc{
		getFn: func(context.Context, int64) (*model.GeoFile, error) { return nil, errors.New("boom") },
	}})
	if _, err := l.GeoEntryList(&types.GeoEntryListReq{Id: "1"}); errCode(err) != "INTERNAL" {
		t.Fatalf("internal code = %q", errCode(err))
	}
}

func TestGeoEntryListPaginatesCategory(t *testing.T) {
	path := writeGeoFile(t, "geosite.txt", "site:example.com\nsite:foo.com\nother:bar.com\n")
	svcCtx := &svc.ServiceContext{GeoModel: geoModelFunc{
		getFn: func(context.Context, int64) (*model.GeoFile, error) { return &model.GeoFile{ID: 1, Path: path}, nil },
	}}
	resp, err := settings.NewGeoEntryListLogic(context.Background(), svcCtx).GeoEntryList(&types.GeoEntryListReq{
		Id: "1", Category: "site", Page: 1, PageSize: 1,
	})
	if err != nil {
		t.Fatalf("GeoEntryList() error = %v", err)
	}
	if resp.Data.Total != 2 || len(resp.Data.Items) != 1 || resp.Data.Items[0].Value != "example.com" || resp.Data.Items[0].Category != "site" {
		t.Fatalf("data = %+v", resp.Data)
	}
}

func TestGeoEntryListPageBeyondEnd(t *testing.T) {
	path := writeGeoFile(t, "geosite.txt", "site:example.com\n")
	svcCtx := &svc.ServiceContext{GeoModel: geoModelFunc{
		getFn: func(context.Context, int64) (*model.GeoFile, error) { return &model.GeoFile{ID: 1, Path: path}, nil },
	}}
	resp, err := settings.NewGeoEntryListLogic(context.Background(), svcCtx).GeoEntryList(&types.GeoEntryListReq{
		Id: "1", Page: 99, PageSize: 50,
	})
	if err != nil {
		t.Fatalf("GeoEntryList() error = %v", err)
	}
	if len(resp.Data.Items) != 0 || resp.Data.Total != 1 {
		t.Fatalf("data = %+v, want empty page with total 1", resp.Data)
	}
}

func TestGeoEntryListUnsupportedFormat(t *testing.T) {
	svcCtx := &svc.ServiceContext{GeoModel: geoModelFunc{
		getFn: func(context.Context, int64) (*model.GeoFile, error) {
			return &model.GeoFile{ID: 1, Path: "data.dat"}, nil
		},
	}}
	if _, err := settings.NewGeoEntryListLogic(context.Background(), svcCtx).GeoEntryList(&types.GeoEntryListReq{Id: "1"}); errCode(err) != "GEO_PARSE_FAILED" {
		t.Fatalf("code = %q, want GEO_PARSE_FAILED", errCode(err))
	}
}

// ---- 通知编辑 ----

func TestNotificationUpdateValidation(t *testing.T) {
	l := settings.NewNotificationUpdateLogic(context.Background(), &svc.ServiceContext{NotificationModel: notificationModelFunc{}})
	if _, err := l.NotificationUpdate(&types.NotificationUpdateReq{Id: "bad"}); errCode(err) != "NOT_FOUND_NOTIFICATION" {
		t.Fatalf("id code = %q", errCode(err))
	}
	badLevel := "debug"
	if _, err := l.NotificationUpdate(&types.NotificationUpdateReq{Id: "1", MinLevel: &badLevel}); errCode(err) != "VALIDATION_INVALID_FIELD" {
		t.Fatalf("level code = %q", errCode(err))
	}
	emptyTarget := "   "
	if _, err := l.NotificationUpdate(&types.NotificationUpdateReq{Id: "1", Target: &emptyTarget}); errCode(err) != "VALIDATION_MISSING_FIELD" {
		t.Fatalf("target code = %q", errCode(err))
	}
	target := "https://hook.example.com"
	if _, err := l.NotificationUpdate(&types.NotificationUpdateReq{Id: "1", Target: &target}); errCode(err) != "INTERNAL" {
		t.Fatalf("box-nil code = %q, want INTERNAL", errCode(err))
	}
}

func TestNotificationUpdateNotFoundAndInternal(t *testing.T) {
	l := settings.NewNotificationUpdateLogic(context.Background(), &svc.ServiceContext{NotificationModel: notificationModelFunc{
		updateFn: func(context.Context, int64, model.NotificationPatch) (*model.NotificationChannel, error) {
			return nil, model.ErrNotFound
		},
	}})
	if _, err := l.NotificationUpdate(&types.NotificationUpdateReq{Id: "1"}); errCode(err) != "NOT_FOUND_NOTIFICATION" {
		t.Fatalf("not-found code = %q", errCode(err))
	}

	l = settings.NewNotificationUpdateLogic(context.Background(), &svc.ServiceContext{NotificationModel: notificationModelFunc{
		updateFn: func(context.Context, int64, model.NotificationPatch) (*model.NotificationChannel, error) {
			return nil, errors.New("boom")
		},
	}})
	if _, err := l.NotificationUpdate(&types.NotificationUpdateReq{Id: "1"}); errCode(err) != "INTERNAL" {
		t.Fatalf("internal code = %q", errCode(err))
	}
}

func TestNotificationUpdateSuccessWithoutTarget(t *testing.T) {
	var gotPatch model.NotificationPatch
	svcCtx := &svc.ServiceContext{NotificationModel: notificationModelFunc{
		updateFn: func(_ context.Context, id int64, p model.NotificationPatch) (*model.NotificationChannel, error) {
			gotPatch = p
			return &model.NotificationChannel{ID: id, Name: "ops", Type: "webhook", MinLevel: "warning", Enabled: true}, nil
		},
	}}
	name := "ops"
	level := "warning"
	enabled := true
	resp, err := settings.NewNotificationUpdateLogic(context.Background(), svcCtx).NotificationUpdate(&types.NotificationUpdateReq{
		Id: "7", Name: &name, MinLevel: &level, Enabled: &enabled,
	})
	if err != nil {
		t.Fatalf("NotificationUpdate() error = %v", err)
	}
	if gotPatch.TargetEncrypted != nil {
		t.Fatal("target must remain untouched when not provided")
	}
	if resp.Data.Item.Id != 7 || resp.Data.Item.Name != "ops" || resp.Data.Item.MinLevel != "warning" {
		t.Fatalf("item = %+v", resp.Data.Item)
	}
}

// ---- 通知测试发送 ----

func TestNotificationTestValidation(t *testing.T) {
	l := settings.NewNotificationTestLogic(context.Background(), &svc.ServiceContext{NotificationModel: notificationModelFunc{}})
	if _, err := l.NotificationTest(&types.NotificationTestReq{Id: "bad"}); errCode(err) != "NOT_FOUND_NOTIFICATION" {
		t.Fatalf("id code = %q", errCode(err))
	}
	if _, err := l.NotificationTest(&types.NotificationTestReq{Id: "1"}); errCode(err) != "NOT_FOUND_NOTIFICATION" {
		t.Fatalf("not-found code = %q", errCode(err))
	}

	l = settings.NewNotificationTestLogic(context.Background(), &svc.ServiceContext{NotificationModel: notificationModelFunc{
		getFn: func(context.Context, int64) (*model.NotificationChannel, error) { return nil, errors.New("boom") },
	}})
	if _, err := l.NotificationTest(&types.NotificationTestReq{Id: "1"}); errCode(err) != "INTERNAL" {
		t.Fatalf("internal code = %q", errCode(err))
	}
}

func TestNotificationTestMissingBox(t *testing.T) {
	svcCtx := &svc.ServiceContext{NotificationModel: notificationModelFunc{
		getFn: func(context.Context, int64) (*model.NotificationChannel, error) {
			return &model.NotificationChannel{ID: 1, Type: "webhook", TargetEncrypted: []byte("sealed")}, nil
		},
	}}
	if _, err := settings.NewNotificationTestLogic(context.Background(), svcCtx).NotificationTest(&types.NotificationTestReq{Id: "1"}); errCode(err) != "INTERNAL" {
		t.Fatalf("code = %q, want INTERNAL (missing master key)", errCode(err))
	}
}

// ---- Geo 文件更新 ----

func TestGeoFileUpdateRejectsInvalidID(t *testing.T) {
	l := settings.NewGeoFileUpdateLogic(context.Background(), &svc.ServiceContext{GeoModel: geoModelFunc{}})
	if _, err := l.GeoFileUpdate(&types.GeoFileReq{Id: "bad"}); errCode(err) != "NOT_FOUND_GEO_FILE" {
		t.Fatalf("code = %q", errCode(err))
	}
}

func TestGeoFileUpdateNotFoundAndInternal(t *testing.T) {
	l := settings.NewGeoFileUpdateLogic(context.Background(), &svc.ServiceContext{GeoModel: geoModelFunc{
		getFn: func(context.Context, int64) (*model.GeoFile, error) { return nil, model.ErrNotFound },
	}})
	if _, err := l.GeoFileUpdate(&types.GeoFileReq{Id: "1"}); errCode(err) != "NOT_FOUND_GEO_FILE" {
		t.Fatalf("not-found code = %q", errCode(err))
	}

	l = settings.NewGeoFileUpdateLogic(context.Background(), &svc.ServiceContext{GeoModel: geoModelFunc{
		getFn: func(context.Context, int64) (*model.GeoFile, error) { return nil, errors.New("boom") },
	}})
	if _, err := l.GeoFileUpdate(&types.GeoFileReq{Id: "1"}); errCode(err) != "INTERNAL" {
		t.Fatalf("internal code = %q", errCode(err))
	}
}

func TestGeoFileUpdateUnsupportedFormat(t *testing.T) {
	svcCtx := &svc.ServiceContext{GeoModel: geoModelFunc{
		getFn: func(context.Context, int64) (*model.GeoFile, error) {
			return &model.GeoFile{ID: 1, Path: "data.dat"}, nil
		},
	}}
	if _, err := settings.NewGeoFileUpdateLogic(context.Background(), svcCtx).GeoFileUpdate(&types.GeoFileReq{Id: "1"}); errCode(err) != "UPSTREAM_UPDATE_SOURCE_FAILED" {
		t.Fatalf("code = %q, want UPSTREAM_UPDATE_SOURCE_FAILED", errCode(err))
	}
}

func TestGeoFileUpdateSuccess(t *testing.T) {
	path := writeGeoFile(t, "geosite.txt", "site:example.com\n")
	var cats []model.GeoCategory
	svcCtx := &svc.ServiceContext{GeoModel: geoModelFunc{
		getFn: func(context.Context, int64) (*model.GeoFile, error) {
			return &model.GeoFile{ID: 3, Kind: "geosite", Name: "geosite", Version: "v2", Path: path}, nil
		},
		replaceCategoriesFn: func(_ context.Context, _ int64, c []model.GeoCategory) error {
			cats = c
			return nil
		},
	}}
	resp, err := settings.NewGeoFileUpdateLogic(context.Background(), svcCtx).GeoFileUpdate(&types.GeoFileReq{Id: "3"})
	if err != nil {
		t.Fatalf("GeoFileUpdate() error = %v", err)
	}
	if resp.Data.State != "ready" || resp.Data.Version != "v2" {
		t.Fatalf("data = %+v", resp.Data)
	}
	if len(cats) != 1 || cats[0].Category != "site" || cats[0].EntriesCount != 1 {
		t.Fatalf("categories = %+v", cats)
	}
}

func TestGeoFileUpdateReplaceCategoriesError(t *testing.T) {
	path := writeGeoFile(t, "geosite.txt", "site:example.com\n")
	svcCtx := &svc.ServiceContext{GeoModel: geoModelFunc{
		getFn: func(context.Context, int64) (*model.GeoFile, error) { return &model.GeoFile{ID: 3, Path: path}, nil },
		replaceCategoriesFn: func(context.Context, int64, []model.GeoCategory) error {
			return errors.New("boom")
		},
	}}
	if _, err := settings.NewGeoFileUpdateLogic(context.Background(), svcCtx).GeoFileUpdate(&types.GeoFileReq{Id: "3"}); errCode(err) != "INTERNAL" {
		t.Fatalf("code = %q, want INTERNAL", errCode(err))
	}
}

// ---- Geo 上传 ----

func TestGeoFileUploadValidation(t *testing.T) {
	l := settings.NewGeoFileUploadLogic(context.Background(), &svc.ServiceContext{})
	if _, err := l.Upload("file.txt", "", nil); errCode(err) != "VALIDATION_MISSING_FIELD" {
		t.Fatalf("empty code = %q", errCode(err))
	}
	if _, err := l.Upload("file.json", "", []byte("x")); errCode(err) != "VALIDATION_INVALID_FILE_TYPE" {
		t.Fatalf("ext code = %q", errCode(err))
	}
	if _, err := l.Upload("file.dat", "", []byte("x")); errCode(err) != "GEO_PARSE_FAILED" {
		t.Fatalf("dat code = %q", errCode(err))
	}
	if _, err := l.Upload("file.txt", "", []byte("# only comment\n")); errCode(err) != "GEO_PARSE_FAILED" {
		t.Fatalf("parse code = %q", errCode(err))
	}
	if _, err := l.GeoFileUpload(); errCode(err) != "VALIDATION_INVALID_FIELD" {
		t.Fatalf("no-body code = %q", errCode(err))
	}
}

func TestGeoFileUploadSuccess(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{}
	cfg.Geo.Dir = dir

	var replaced []model.GeoCategory
	svcCtx := &svc.ServiceContext{
		Config: cfg,
		GeoModel: geoModelFunc{
			upsertFn: func(_ context.Context, f model.GeoFile) (*model.GeoFile, error) {
				f.ID = 9
				return &f, nil
			},
			replaceCategoriesFn: func(_ context.Context, _ int64, cats []model.GeoCategory) error {
				replaced = cats
				return nil
			},
			getFn: func(context.Context, int64) (*model.GeoFile, error) {
				return &model.GeoFile{ID: 9, Kind: "geosite", Name: "custom.txt", Version: "20260101", Status: "ready"}, nil
			},
		},
	}
	resp, err := settings.NewGeoFileUploadLogic(context.Background(), svcCtx).Upload("custom.txt", "", []byte("site:example.com\nsite:foo.com\n"))
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if resp.Data.Id != 9 || resp.Data.Name != "custom.txt" || resp.Data.Kind != "geosite" {
		t.Fatalf("data = %+v", resp.Data)
	}
	if len(replaced) != 1 || replaced[0].EntriesCount != 2 {
		t.Fatalf("categories = %+v", replaced)
	}
}

func TestGeoFileUploadUpsertError(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{}
	cfg.Geo.Dir = dir
	svcCtx := &svc.ServiceContext{
		Config: cfg,
		GeoModel: geoModelFunc{
			upsertFn: func(context.Context, model.GeoFile) (*model.GeoFile, error) { return nil, errors.New("boom") },
		},
	}
	if _, err := settings.NewGeoFileUploadLogic(context.Background(), svcCtx).Upload("custom.txt", "", []byte("site:example.com\n")); errCode(err) != "INTERNAL" {
		t.Fatalf("code = %q, want INTERNAL", errCode(err))
	}
}
