// Package middleware holds admin-api HTTP middlewares that wrap the generated
// go-zero handler chain.
package middleware

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

const (
	apiTokenPrefix    = "ota_pat_"
	bearerPrefix      = "Bearer "
	bridgeTokenExpire = 5 * time.Minute
)

var errInvalidApiToken = httperr.New(http.StatusUnauthorized, "invalid or expired API token")

// ApiTokenAuthMiddleware lets CLI/CI authenticate the management API with a
// long-lived API token (§6). It runs before the JWT middleware: when it sees a
// `Bearer ota_pat_...` credential it validates the token against the database
// and rewrites the Authorization header into a freshly-minted, short-lived
// access JWT for the token's creating user, so every downstream handler (and
// the audit actor) behaves exactly as for an interactive admin. Requests
// without an API token pass through untouched for the JWT middleware to handle.
type ApiTokenAuthMiddleware struct {
	svcCtx *svc.ServiceContext
}

func NewApiTokenAuthMiddleware(svcCtx *svc.ServiceContext) *ApiTokenAuthMiddleware {
	return &ApiTokenAuthMiddleware{svcCtx: svcCtx}
}

func (m *ApiTokenAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), bearerPrefix)
		if !strings.HasPrefix(raw, apiTokenPrefix) {
			next(w, r)
			return
		}

		bridged, err := m.exchange(r.Context(), raw)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		r.Header.Set("Authorization", bearerPrefix+bridged)
		next(w, r)
	}
}

// exchange validates the API token and returns a short-lived access JWT for the
// token's creating user.
func (m *ApiTokenAuthMiddleware) exchange(ctx context.Context, plaintext string) (string, error) {
	sum := sha256.Sum256([]byte(plaintext))
	token, err := m.svcCtx.ApiTokensModel.FindOneByTokenHash(ctx, models.ByteaHex(sum[:]))
	if err != nil {
		return "", errInvalidApiToken
	}
	if token.RevokedAt.Valid {
		return "", errInvalidApiToken
	}
	if token.ExpiresAt.Valid && !token.ExpiresAt.Time.After(time.Now()) {
		return "", errInvalidApiToken
	}

	// Best-effort last-used bookkeeping; failures never block the request.
	token.LastUsedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	if uerr := m.svcCtx.ApiTokensModel.Update(ctx, token); uerr != nil {
		logx.WithContext(ctx).Errorf("update api token last_used_at failed: %v", uerr)
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"userId": token.CreatedBy,
		"typ":    "access",
		"iat":    now.Unix(),
		"exp":    now.Add(bridgeTokenExpire).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(m.svcCtx.Config.Auth.AccessSecret))
}
