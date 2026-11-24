// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type GeoFileListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// Geo 文件列表
func NewGeoFileListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GeoFileListLogic {
	return &GeoFileListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GeoFileListLogic) GeoFileList(req *types.GeoFileListReq) (resp *types.GeoFileListResp, err error) {
	files, total, err := l.svcCtx.GeoModel.List(l.ctx, req.Kind, req.Page, req.PageSize)
	if err != nil {
		return nil, response.Internal(err)
	}
	items := make([]types.GeoFile, 0, len(files))
	for _, f := range files {
		items = append(items, toGeoFile(f))
	}
	return &types.GeoFileListResp{
		Code:      errcode.OK,
		Data:      types.GeoFileListData{Items: items, Total: int(total), Page: req.Page, PageSize: req.PageSize},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
