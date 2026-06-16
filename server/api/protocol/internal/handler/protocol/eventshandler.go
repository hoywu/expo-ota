// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package protocol

import (
	"net/http"

	"github.com/hoywu/expo-ota/server/api/protocol/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/protocol/internal/logic/protocol"
	"github.com/hoywu/expo-ota/server/api/protocol/internal/svc"
	"github.com/hoywu/expo-ota/server/api/protocol/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func EventsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.EventReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, httperr.BadRequest(err))
			return
		}

		l := protocol.NewEventsLogic(r.Context(), svcCtx)
		resp, err := l.Events(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
