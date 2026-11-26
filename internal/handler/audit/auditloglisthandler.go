// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package audit

import (
	"net/http"

	"github.com/relayx-tech/panel/internal/logic/audit"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 审计列表
func AuditLogListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AuditLogListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := audit.NewAuditLogListLogic(r.Context(), svcCtx)
		resp, err := l.AuditLogList(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
