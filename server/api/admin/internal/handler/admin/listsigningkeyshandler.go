package admin

import (
	"net/http"

	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/logic/admin"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ListSigningKeysHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AppSlugPath
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, httperr.BadRequest(err))
			return
		}

		l := admin.NewListSigningKeysLogic(r.Context(), svcCtx)
		resp, err := l.ListSigningKeys(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
