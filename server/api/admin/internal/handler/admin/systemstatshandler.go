// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"net/http"

	"github.com/hoywu/expo-ota/server/api/admin/internal/logic/admin"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func SystemStatsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := admin.NewSystemStatsLogic(r.Context(), svcCtx)
		resp, err := l.SystemStats()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
