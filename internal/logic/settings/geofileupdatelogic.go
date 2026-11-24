// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/geo"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type GeoFileUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 检查并更新 Geo 文件
func NewGeoFileUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GeoFileUpdateLogic {
	return &GeoFileUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GeoFileUpdateLogic) GeoFileUpdate(req *types.GeoFileReq) (resp *types.GeoFileUpdateResp, err error) {
	id, perr := strconv.ParseInt(req.Id, 10, 64)
	if perr != nil || id <= 0 {
		return nil, response.NewError(errcode.NOT_FOUND_GEO_FILE, "geo file not found")
	}
	file, err := l.svcCtx.GeoModel.Get(l.ctx, id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, response.NewError(errcode.NOT_FOUND_GEO_FILE, "geo file not found")
		}
		return nil, response.Internal(err)
	}

	// 本地重新解析校验并重建索引；更新源下载由部署侧镜像负责（配置未提供时明确失败）。
	ds, perr2 := geo.Parse(file.Path)
	if perr2 != nil {
		return nil, response.NewError(errcode.UPSTREAM_UPDATE_SOURCE_FAILED, perr2.Error())
	}
	cats := make([]model.GeoCategory, 0, len(ds.Categories))
	for _, c := range ds.Categories {
		cats = append(cats, model.GeoCategory{Category: c.Name, EntriesCount: c.EntriesCount})
	}
	if err := l.svcCtx.GeoModel.ReplaceCategories(l.ctx, id, cats); err != nil {
		return nil, response.Internal(err)
	}
	if _, err := l.svcCtx.GeoModel.Upsert(l.ctx, model.GeoFile{
		Kind: file.Kind, Name: file.Name, Version: file.Version, SizeBytes: file.SizeBytes,
		Checksum: file.Checksum, Status: "ready", Path: file.Path,
	}); err != nil {
		return nil, response.Internal(err)
	}
	return &types.GeoFileUpdateResp{
		Code:      errcode.OK,
		Data:      types.GeoFileUpdateData{State: "ready", Version: file.Version},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}

// checksumOf 计算文件内容 SHA-256（上传时使用）。
func checksumOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
