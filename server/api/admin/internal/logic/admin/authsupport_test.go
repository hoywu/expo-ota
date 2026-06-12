package admin

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/hoywu/expo-ota/server/api/admin/internal/config"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/db/models"
	"go.uber.org/mock/gomock"
)

const (
	testAccessSecret  = "test-access-secret"
	testRefreshSecret = "test-refresh-secret"
	testAccessExpire  = int64(900)
	testRefreshExpire = int64(86400)
)

func newTestSvcCtx(ctrl *gomock.Controller) (*svc.ServiceContext, *models.MockUsersModel) {
	usersModel := models.NewMockUsersModel(ctrl)

	c := config.Config{
		RefreshSecret: testRefreshSecret,
		RefreshExpire: testRefreshExpire,
	}
	c.Auth.AccessSecret = testAccessSecret
	c.Auth.AccessExpire = testAccessExpire

	return &svc.ServiceContext{
		Config:     c,
		UsersModel: usersModel,
	}, usersModel
}

func newTestUser() *models.Users {
	return &models.Users{
		Id:        "user-1",
		Username:  "alice",
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

// parseTestToken parses a token signed with the given secret and returns its claims.
func parseTestToken(t *testing.T, secret, tokenString string) jwt.MapClaims {
	t.Helper()

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		t.Fatalf("token is not valid: %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("claims are not MapClaims: %T", token.Claims)
	}

	return claims
}

func disabledAt() sql.NullTime {
	return sql.NullTime{Time: time.Now().UTC(), Valid: true}
}

func ctxWithUserID(userID string) context.Context {
	return context.WithValue(context.Background(), "userId", userID) //nolint:staticcheck // go-zero JWT middleware uses a string key
}
