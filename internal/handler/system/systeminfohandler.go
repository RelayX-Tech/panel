// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package system

import (
	"net/http"

	"github.com/relayx-tech/panel/internal/logic/system"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 版本与兼容信息
func SystemInfoHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := system.NewSystemInfoLogic(r.Context(), svcCtx)
		resp, err := l.SystemInfo()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
