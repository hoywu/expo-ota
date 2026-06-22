// Package middleware holds admin-api HTTP middlewares that wrap the generated
// go-zero handler chain.
package middleware

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

const (
	apiTokenPrefix = "ota_pat_"
	bearerPrefix   = "Bearer "
)

var errInvalidApiToken = httperr.New(http.StatusUnauthorized, "invalid or expired API token")
var errApiTokenForbidden = httperr.New(http.StatusForbidden, "API token is not allowed to access this endpoint")

// ApiTokenAuthMiddleware lets CLI/CI authenticate plan/finalize upload routes
// with a long-lived, app-scoped API token (§6). Publish remains dashboard-only
// via admin JWT.
type ApiTokenAuthMiddleware struct {
	svcCtx *svc.ServiceContext
}

func NewApiTokenAuthMiddleware(svcCtx *svc.ServiceContext) *ApiTokenAuthMiddleware {
	return &ApiTokenAuthMiddleware{svcCtx: svcCtx}
}

func (m *ApiTokenAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), bearerPrefix)
		ciUpload := isCiUploadPath(r.Method, r.URL.Path)

		if !strings.HasPrefix(raw, apiTokenPrefix) {
			if ciUpload {
				httpx.ErrorCtx(r.Context(), w, errInvalidApiToken)
				return
			}
			next(w, r)
			return
		}

		if !ciUpload {
			httpx.ErrorCtx(r.Context(), w, errApiTokenForbidden)
			return
		}

		token, err := m.validate(r.Context(), r, raw)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		ctx := context.WithValue(r.Context(), "userId", token.CreatedBy)
		next(w, r.WithContext(ctx))
	}
}

func (m *ApiTokenAuthMiddleware) validate(ctx context.Context, r *http.Request, plaintext string) (*models.ApiTokens, error) {
	sum := sha256.Sum256([]byte(plaintext))
	token, err := m.svcCtx.ApiTokensModel.FindOneByTokenHash(ctx, sum[:])
	if err != nil {
		if !errors.Is(err, models.ErrNotFound) {
			logc.Errorf(ctx, "lookup api token by hash failed: %v", err)
		}
		return nil, errInvalidApiToken
	}
	if token.RevokedAt.Valid {
		logc.Infof(ctx, "revoked api token %s used", token.Id)
		return nil, errInvalidApiToken
	}
	if token.ExpiresAt.Valid && !token.ExpiresAt.Time.After(time.Now()) {
		logc.Infof(ctx, "expired api token %s used", token.Id)
		return nil, errInvalidApiToken
	}
	if err := m.authorize(ctx, r, token); err != nil {
		return nil, err
	}

	token.LastUsedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	if uerr := m.svcCtx.ApiTokensModel.Update(ctx, token); uerr != nil {
		logc.Errorf(ctx, "update api token last_used_at failed: %v", uerr)
	}

	return token, nil
}

// authorize enforces the current API token scope model: tokens with the
// "publish" scope may only operate on their own app's CI upload endpoints.
func (m *ApiTokenAuthMiddleware) authorize(ctx context.Context, r *http.Request, token *models.ApiTokens) error {
	appSlug, ok := ciUploadAppSlug(r.Method, r.URL.Path)
	if !ok || !hasScope(token.Scopes, "publish") {
		return errApiTokenForbidden
	}

	app, err := m.svcCtx.AppsModel.FindOneByAppSlug(ctx, appSlug)
	if err != nil {
		if !errors.Is(err, models.ErrNotFound) {
			logc.Errorf(ctx, "lookup app by slug during api token auth failed: %v", err)
		}
		return errApiTokenForbidden
	}
	if app.DeletedAt.Valid || app.Id != token.AppId {
		logc.Infof(ctx, "api token %s for app %s denied access to app %s", token.Id, token.AppId, appSlug)
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

func isCiUploadPath(method, path string) bool {
	_, ok := ciUploadAppSlug(method, path)
	return ok
}

func ciUploadAppSlug(method, path string) (string, bool) {
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
	return "", false
}
