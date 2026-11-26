// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
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

type GeoFileValidateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 校验 Geo 文件
func NewGeoFileValidateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GeoFileValidateLogic {
	return &GeoFileValidateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GeoFileValidateLogic) GeoFileValidate(req *types.GeoFileReq) (resp *types.GeoValidateResp, err error) {
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
	ds, perr2 := geo.Parse(file.Path)
	if perr2 != nil {
		return &types.GeoValidateResp{
			Code:      errcode.OK,
			Data:      types.GeoValidateData{Valid: false, Error: perr2.Error()},
			RequestId: requestid.FromContext(l.ctx),
		}, nil
	}
	entries := 0
	for _, c := range ds.Categories {
		entries += c.EntriesCount
	}
	return &types.GeoValidateResp{
		Code:      errcode.OK,
		Data:      types.GeoValidateData{Valid: true, CategoriesCount: len(ds.Categories), EntriesCount: entries},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
