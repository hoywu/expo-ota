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

// ManifestHandler renders the protocol-defined manifest response directly to
// the ResponseWriter (multipart/mixed, application/expo+json, 204, or 406
// with expo-* headers), which the default JSON renderer cannot express.
func ManifestHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ManifestReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, httperr.BadRequest(err))
			return
		}

		l := protocol.NewManifestLogic(r.Context(), svcCtx)
		res, err := l.Manifest(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		for key, values := range res.Header {
			for _, v := range values {
				w.Header().Add(key, v)
			}
		}
		w.WriteHeader(res.StatusCode)
		if len(res.Body) > 0 {
			_, _ = w.Write(res.Body)
		}
	}
}
