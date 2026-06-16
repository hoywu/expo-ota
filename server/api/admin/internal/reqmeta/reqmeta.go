// Package reqmeta captures per-request metadata (client IP, User-Agent,
// request id) into the context so the logic layer can attach it to audit
// logs without depending on *http.Request.
package reqmeta

import (
	"context"
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
)

type ctxKey struct{}

// Meta is the request metadata recorded in audit logs (§11.5).
type Meta struct {
	IP        string
	UserAgent string
	RequestID string
}

// Middleware stores the request metadata in the request context.
func Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		meta := Meta{
			IP:        httpx.GetRemoteAddr(r),
			UserAgent: r.UserAgent(),
			RequestID: r.Header.Get("X-Request-Id"),
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, meta)))
	}
}

// FromContext returns the request metadata, if recorded.
func FromContext(ctx context.Context) (Meta, bool) {
	meta, ok := ctx.Value(ctxKey{}).(Meta)
	return meta, ok
}
