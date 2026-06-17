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
var errApiTokenForbidden = httperr.New(http.StatusForbidden, "API token is not allowed to access this endpoint")

// ApiTokenAuthMiddleware lets CLI/CI authenticate the management API with a
// long-lived, app-scoped API token (§6). It runs before the JWT middleware:
// when it sees a `Bearer ota_pat_...` credential it validates the token, checks
// that the request is a publish-scope action for the token's app, then rewrites
// the Authorization header into a short-lived access JWT for the token's
// creating user. Requests without an API token pass through untouched for the
// JWT middleware to handle.
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

		bridged, err := m.exchange(r.Context(), r, raw)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		r.Header.Set("Authorization", bearerPrefix+bridged)
		next(w, r)
	}
}

// exchange validates and authorizes the API token, then returns a short-lived
// access JWT for the token's creating user.
func (m *ApiTokenAuthMiddleware) exchange(ctx context.Context, r *http.Request, plaintext string) (string, error) {
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
	if err := m.authorize(ctx, r, token); err != nil {
		return "", err
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

// authorize enforces the current API token scope model: tokens with the
// "publish" scope may only operate on their own app's publish endpoints.
func (m *ApiTokenAuthMiddleware) authorize(ctx context.Context, r *http.Request, token *models.ApiTokens) error {
	appSlug, ok := publishAppSlug(r.Method, r.URL.Path)
	if !ok || !hasScope(token.Scopes, "publish") {
		return errApiTokenForbidden
	}

	app, err := m.svcCtx.AppsModel.FindOneByAppSlug(ctx, appSlug)
	if err != nil {
		return errApiTokenForbidden
	}
	if app.DeletedAt.Valid || app.Id != token.AppId {
		return errApiTokenForbidden
	}
	return nil
}

func hasScope(scopes []string, want string) bool {
	for _, scope := range scopes {
		if scope == want {
			return true
		}
	}
	return false
}

func publishAppSlug(method, path string) (string, bool) {
	if method != http.MethodPost {
		return "", false
	}

	const prefix = "/api/admin/apps/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(path, prefix), "/"), "/")
	if len(parts) == 3 && parts[1] == "uploads" && (parts[2] == "plan" || parts[2] == "finalize") {
		return parts[0], parts[0] != ""
	}
	if len(parts) == 4 && parts[1] == "updates" && parts[3] == "publish" {
		return parts[0], parts[0] != ""
	}
	return "", false
}
