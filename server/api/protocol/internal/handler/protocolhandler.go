// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package handler

import (
	"net/http"

	"github.com/hoywu/expo-ota/server/api/protocol/internal/logic"
	"github.com/hoywu/expo-ota/server/api/protocol/internal/svc"
	"github.com/hoywu/expo-ota/server/api/protocol/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ProtocolHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.Request
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewProtocolLogic(r.Context(), svcCtx)
		resp, err := l.Protocol(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
