// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"io"
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"github.com/relayx-tech/panel/internal/logic/settings"
	"github.com/relayx-tech/panel/internal/svc"
)

// 上传自定义 Geo 文件（multipart/form-data）
func GeoFileUploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 100<<20)
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			httpx.ErrorCtx(r.Context(), w, settings.InvalidMultipart(err))
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, settings.InvalidMultipart(err))
			return
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, settings.InvalidMultipart(err))
			return
		}

		l := settings.NewGeoFileUploadLogic(r.Context(), svcCtx)
		resp, err := l.Upload(header.Filename, r.FormValue("source"), data)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
