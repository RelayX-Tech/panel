// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
	"errors"
	"strconv"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type GeoCategoryListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// Geo 分类浏览
func NewGeoCategoryListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GeoCategoryListLogic {
	return &GeoCategoryListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GeoCategoryListLogic) GeoCategoryList(req *types.GeoCategoryListReq) (resp *types.GeoCategoryListResp, err error) {
	id, perr := strconv.ParseInt(req.Id, 10, 64)
	if perr != nil || id <= 0 {
		return nil, response.NewError(errcode.NOT_FOUND_GEO_FILE, "geo file not found")
	}
	if _, err := l.svcCtx.GeoModel.Get(l.ctx, id); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, response.NewError(errcode.NOT_FOUND_GEO_FILE, "geo file not found")
		}
		return nil, response.Internal(err)
	}
	cats, total, err := l.svcCtx.GeoModel.Categories(l.ctx, id, req.Page, req.PageSize)
	if err != nil {
		return nil, response.Internal(err)
	}
	items := make([]types.GeoCategory, 0, len(cats))
	for _, c := range cats {
		items = append(items, types.GeoCategory{Category: c.Category, EntriesCount: c.EntriesCount})
	}
	return &types.GeoCategoryListResp{
		Code:      errcode.OK,
		Data:      types.GeoCategoryListData{Items: items, Total: int(total), Page: req.Page, PageSize: req.PageSize},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
