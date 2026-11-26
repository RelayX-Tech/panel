// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/geo"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

const maxGeoUploadBytes = 100 << 20 // 100MB

type GeoFileUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 上传自定义 Geo 文件（multipart/form-data）
func NewGeoFileUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GeoFileUploadLogic {
	return &GeoFileUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GeoFileUpload 不支持无 body 调用；文件处理经 Upload。
func (l *GeoFileUploadLogic) GeoFileUpload() (resp *types.GeoUploadResp, err error) {
	return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "file", "multipart file is required")
}

// Upload 校验并落盘自定义 Geo 文件，解析成功后写入索引。
func (l *GeoFileUploadLogic) Upload(filename, source string, data []byte) (*types.GeoUploadResp, error) {
	if len(data) == 0 {
		return nil, response.MissingField("file")
	}
	if len(data) > maxGeoUploadBytes {
		return nil, response.NewError(errcode.VALIDATION_FILE_TOO_LARGE, "file exceeds 100MB")
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".dat" && ext != ".txt" {
		return nil, response.NewError(errcode.VALIDATION_INVALID_FILE_TYPE, "only .dat or .txt is allowed")
	}
	if !geo.Supported(filename) {
		return nil, response.NewError(errcode.GEO_PARSE_FAILED, "binary .dat requires Xray baseline reader")
	}
	ds, err := geo.ParseBytes(data)
	if err != nil {
		return nil, response.NewError(errcode.GEO_PARSE_FAILED, err.Error())
	}

	dir := l.svcCtx.Config.Geo.Dir
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "relayx-geo")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, response.Internal(err)
	}
	path := filepath.Join(dir, uuid.NewString()+ext)
	if err := os.WriteFile(path, data, 0o640); err != nil {
		return nil, response.Internal(err)
	}

	created, err := l.svcCtx.GeoModel.Upsert(l.ctx, model.GeoFile{
		Kind: "geosite", Name: filepath.Base(filename), Version: time.Now().UTC().Format("20060102"),
		SizeBytes: int64(len(data)), Checksum: checksumOf(data), Status: "ready", Path: path,
	})
	if err != nil {
		_ = os.Remove(path)
		return nil, response.Internal(err)
	}
	cats := make([]model.GeoCategory, 0, len(ds.Categories))
	for _, c := range ds.Categories {
		cats = append(cats, model.GeoCategory{Category: c.Name, EntriesCount: c.EntriesCount})
	}
	if err := l.svcCtx.GeoModel.ReplaceCategories(l.ctx, created.ID, cats); err != nil {
		return nil, response.Internal(err)
	}
	updated, err := l.svcCtx.GeoModel.Get(l.ctx, created.ID)
	if err != nil {
		return nil, response.Internal(err)
	}
	_ = source
	return &types.GeoUploadResp{
		Code:      errcode.OK,
		Data:      toGeoFile(*updated),
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
