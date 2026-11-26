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

type GeoEntryListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// Geo 条目查询
func NewGeoEntryListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GeoEntryListLogic {
	return &GeoEntryListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GeoEntryListLogic) GeoEntryList(req *types.GeoEntryListReq) (resp *types.GeoEntryListResp, err error) {
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
	ds, err := geo.Parse(file.Path)
	if err != nil {
		return nil, response.NewError(errcode.GEO_PARSE_FAILED, err.Error())
	}
	all := ds.EntriesOf(req.Category)
	page, pageSize := req.Page, req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	start := (page - 1) * pageSize
	if start > len(all) {
		start = len(all)
	}
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}
	items := make([]types.GeoEntry, 0, end-start)
	for _, v := range all[start:end] {
		items = append(items, types.GeoEntry{Value: v, Category: req.Category})
	}
	return &types.GeoEntryListResp{
		Code:      errcode.OK,
		Data:      types.GeoEntryListData{Items: items, Total: len(all), Page: page, PageSize: pageSize},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
