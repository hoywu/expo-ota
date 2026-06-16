// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package protocol

import (
	"net/http"

	"github.com/hoywu/expo-ota/server/api/protocol/internal/logic/protocol"
	"github.com/hoywu/expo-ota/server/api/protocol/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func ReadyzHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := protocol.NewReadyzLogic(r.Context(), svcCtx)
		resp, err := l.Readyz()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
