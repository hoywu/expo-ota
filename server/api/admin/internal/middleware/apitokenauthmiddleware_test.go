package middleware

import (
	"crypto/sha256"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/hoywu/expo-ota/server/api/admin/internal/config"
	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/rest/httpx"
	"go.uber.org/mock/gomock"
)

const testSecret = "test-access-secret-0123456789abcdef"

func init() {
	// Match main(): map domain errors to their HTTP status codes.
	httpx.SetErrorHandlerCtx(httperr.ToErrorResponse)
}

func newMwSvcCtx(tokens models.ApiTokensModel) *svc.ServiceContext {
	c := config.Config{}
	c.Auth.AccessSecret = testSecret
	return &svc.ServiceContext{Config: c, ApiTokensModel: tokens}
}

func hashHex(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return models.ByteaHex(sum[:])
}

func runHandle(t *testing.T, tokens models.ApiTokensModel, authHeader string) (called bool, gotAuth string, rec *httptest.ResponseRecorder) {
	t.Helper()
	mw := NewApiTokenAuthMiddleware(newMwSvcCtx(tokens))
	next := func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotAuth = r.Header.Get("Authorization")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/apps", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec = httptest.NewRecorder()
	mw.Handle(next)(rec, req)
	return called, gotAuth, rec
}

func TestApiTokenAuthBridgesValidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	tokens := models.NewMockApiTokensModel(ctrl)

	plaintext := "ota_pat_validtoken"
	tokens.EXPECT().FindOneByTokenHash(gomock.Any(), hashHex(plaintext)).
		Return(&models.ApiTokens{Id: "tok-1", CreatedBy: "user-9"}, nil)
	tokens.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	called, gotAuth, _ := runHandle(t, tokens, "Bearer "+plaintext)
	if !called {
		t.Fatal("next was not called")
	}
	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Fatalf("Authorization not rewritten: %q", gotAuth)
	}

	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(strings.TrimPrefix(gotAuth, "Bearer "), claims,
		func(*jwt.Token) (any, error) { return []byte(testSecret), nil }); err != nil {
		t.Fatalf("minted JWT did not parse: %v", err)
	}
	if claims["userId"] != "user-9" {
		t.Errorf("userId claim = %v, want user-9", claims["userId"])
	}
}

func TestApiTokenAuthRejectsRevoked(t *testing.T) {
	ctrl := gomock.NewController(t)
	tokens := models.NewMockApiTokensModel(ctrl)

	plaintext := "ota_pat_revoked"
	tokens.EXPECT().FindOneByTokenHash(gomock.Any(), hashHex(plaintext)).
		Return(&models.ApiTokens{RevokedAt: sql.NullTime{Time: time.Now(), Valid: true}}, nil)

	called, _, rec := runHandle(t, tokens, "Bearer "+plaintext)
	if called {
		t.Fatal("next should not be called for a revoked token")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestApiTokenAuthRejectsExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	tokens := models.NewMockApiTokensModel(ctrl)

	plaintext := "ota_pat_expired"
	tokens.EXPECT().FindOneByTokenHash(gomock.Any(), hashHex(plaintext)).
		Return(&models.ApiTokens{ExpiresAt: sql.NullTime{Time: time.Now().Add(-time.Hour), Valid: true}}, nil)

	called, _, rec := runHandle(t, tokens, "Bearer "+plaintext)
	if called {
		t.Fatal("next should not be called for an expired token")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestApiTokenAuthRejectsUnknown(t *testing.T) {
	ctrl := gomock.NewController(t)
	tokens := models.NewMockApiTokensModel(ctrl)

	plaintext := "ota_pat_unknown"
	tokens.EXPECT().FindOneByTokenHash(gomock.Any(), hashHex(plaintext)).
		Return(nil, models.ErrNotFound)

	called, _, rec := runHandle(t, tokens, "Bearer "+plaintext)
	if called {
		t.Fatal("next should not be called for an unknown token")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestApiTokenAuthPassesThroughJwt(t *testing.T) {
	ctrl := gomock.NewController(t)
	tokens := models.NewMockApiTokensModel(ctrl)
	// No EXPECT: a non-ota_pat bearer must not touch the token store.

	jwtHeader := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig"
	called, gotAuth, _ := runHandle(t, tokens, jwtHeader)
	if !called {
		t.Fatal("next was not called for a JWT bearer")
	}
	if gotAuth != jwtHeader {
		t.Errorf("Authorization mutated: %q, want %q", gotAuth, jwtHeader)
	}
}
