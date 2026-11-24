// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package ws

import (
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/svc"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		// 管理面同源部署；Origin 校验已由 OriginCheck 中间件在下发前完成。
		return true
	},
}

// WebSocket 实时通道
func WsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logx.WithContext(r.Context()).Errorw("ws.upgrade.failure", logx.Field("error", err.Error()))
			return
		}
		resources := splitResources(r.URL.Query().Get("resources"))
		if svcCtx.WSHub == nil {
			_ = conn.Close()
			return
		}
		svcCtx.WSHub.Serve(conn, resources)
	}
}

func splitResources(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
