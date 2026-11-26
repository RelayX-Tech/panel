// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"net/http"

	"github.com/relayx-tech/panel/internal/logic/settings"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 检查并更新 Geo 文件
func GeoFileUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GeoFileReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := settings.NewGeoFileUpdateLogic(r.Context(), svcCtx)
		resp, err := l.GeoFileUpdate(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
