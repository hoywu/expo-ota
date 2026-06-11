package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
)

var (
	errInvalidCredentials = httperr.New(http.StatusUnauthorized, "invalid username or password")
	errUserDisabled       = httperr.New(http.StatusForbidden, "user is disabled")
	errUnauthorized       = httperr.New(http.StatusUnauthorized, "unauthorized")
)

const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

func newAccessToken(secret string, expireSeconds int64, user *models.Users) (string, error) {
	return newUserToken(secret, expireSeconds, user, tokenTypeAccess)
}

func newRefreshToken(secret string, expireSeconds int64, user *models.Users) (string, error) {
	return newUserToken(secret, expireSeconds, user, tokenTypeRefresh)
}

func newUserToken(secret string, expireSeconds int64, user *models.Users, tokenType string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"userId":   user.Id,
		"username": user.Username,
		"typ":      tokenType,
		"iat":      now.Unix(),
		"exp":      now.Add(time.Duration(expireSeconds) * time.Second).Unix(),
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func userToMeResp(user *models.Users) *types.MeResp {
	resp := &types.MeResp{
		UserId:    user.Id,
		Username:  user.Username,
		CreatedAt: user.CreatedAt.UTC().Format(time.RFC3339),
	}
	if user.LastLoginAt.Valid {
		resp.LastLoginAt = user.LastLoginAt.Time.UTC().Format(time.RFC3339)
	}

	return resp
}

func tokensToLoginResp(accessToken, refreshToken string, expiresIn int64) *types.LoginResp {
	return &types.LoginResp{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}
}

func tokensToRefreshResp(accessToken, refreshToken string, expiresIn int64) *types.RefreshResp {
	return &types.RefreshResp{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}
}

func parseRefreshToken(secret, tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errUnauthorized
		}

		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, errUnauthorized
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errUnauthorized
	}

	if claims["typ"] != tokenTypeRefresh {
		return nil, errUnauthorized
	}

	return claims, nil
}

func userIDFromClaims(claims jwt.MapClaims) (string, error) {
	value, ok := claims["userId"]
	if !ok {
		return "", errUnauthorized
	}

	switch v := value.(type) {
	case string:
		if v == "" {
			return "", errUnauthorized
		}
		return v, nil
	default:
		return "", errUnauthorized
	}
}

func userIDFromContext(ctx context.Context) (string, error) {
	value := ctx.Value("userId")
	switch v := value.(type) {
	case string:
		if v == "" {
			return "", errUnauthorized
		}
		return v, nil
	case json.Number:
		return v.String(), nil
	case nil:
		return "", errUnauthorized
	default:
		return "", errUnauthorized
	}
}

func validateActiveUser(user *models.Users) error {
	if user == nil {
		return errUnauthorized
	}
	if user.DisabledAt.Valid {
		return errUserDisabled
	}

	return nil
}

func nullableNow() sql.NullTime {
	return sql.NullTime{Time: time.Now().UTC(), Valid: true}
}
