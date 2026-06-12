package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
	"go.uber.org/mock/gomock"
)

// signRefreshToken issues a refresh token the same way the server does.
func signRefreshToken(t *testing.T, secret string, expireSeconds int64, user *models.Users) string {
	t.Helper()

	token, err := newRefreshToken(secret, expireSeconds, user)
	if err != nil {
		t.Fatalf("newRefreshToken: %v", err)
	}

	return token
}

func TestRefreshSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, usersModel := newTestSvcCtx(ctrl)
	user := newTestUser()

	usersModel.EXPECT().
		FindOne(gomock.Any(), user.Id).
		Return(user, nil)

	resp, err := NewRefreshLogic(context.Background(), svcCtx).Refresh(&types.RefreshReq{
		RefreshToken: signRefreshToken(t, testRefreshSecret, testRefreshExpire, user),
	})
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}

	if resp.ExpiresIn != testAccessExpire {
		t.Errorf("ExpiresIn = %d, want %d", resp.ExpiresIn, testAccessExpire)
	}

	accessClaims := parseTestToken(t, testAccessSecret, resp.AccessToken)
	if accessClaims["typ"] != tokenTypeAccess {
		t.Errorf("access token typ = %v, want %q", accessClaims["typ"], tokenTypeAccess)
	}
	if accessClaims["userId"] != user.Id {
		t.Errorf("access token userId = %v, want %q", accessClaims["userId"], user.Id)
	}

	refreshClaims := parseTestToken(t, testRefreshSecret, resp.RefreshToken)
	if refreshClaims["typ"] != tokenTypeRefresh {
		t.Errorf("refresh token typ = %v, want %q", refreshClaims["typ"], tokenTypeRefresh)
	}
}

func TestRefreshMalformedToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, _ := newTestSvcCtx(ctrl)

	_, err := NewRefreshLogic(context.Background(), svcCtx).Refresh(&types.RefreshReq{
		RefreshToken: "not-a-jwt",
	})
	if !errors.Is(err, errUnauthorized) {
		t.Errorf("err = %v, want errUnauthorized", err)
	}
}

func TestRefreshWrongSecret(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, _ := newTestSvcCtx(ctrl)

	_, err := NewRefreshLogic(context.Background(), svcCtx).Refresh(&types.RefreshReq{
		RefreshToken: signRefreshToken(t, "other-secret", testRefreshExpire, newTestUser()),
	})
	if !errors.Is(err, errUnauthorized) {
		t.Errorf("err = %v, want errUnauthorized", err)
	}
}

func TestRefreshExpiredToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, _ := newTestSvcCtx(ctrl)

	_, err := NewRefreshLogic(context.Background(), svcCtx).Refresh(&types.RefreshReq{
		RefreshToken: signRefreshToken(t, testRefreshSecret, -60, newTestUser()),
	})
	if !errors.Is(err, errUnauthorized) {
		t.Errorf("err = %v, want errUnauthorized", err)
	}
}

func TestRefreshRejectsAccessToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, _ := newTestSvcCtx(ctrl)

	// An access token signed with the refresh secret must still be rejected by typ.
	accessToken, err := newAccessToken(testRefreshSecret, testAccessExpire, newTestUser())
	if err != nil {
		t.Fatalf("newAccessToken: %v", err)
	}

	_, err = NewRefreshLogic(context.Background(), svcCtx).Refresh(&types.RefreshReq{
		RefreshToken: accessToken,
	})
	if !errors.Is(err, errUnauthorized) {
		t.Errorf("err = %v, want errUnauthorized", err)
	}
}

func TestRefreshUserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, usersModel := newTestSvcCtx(ctrl)
	user := newTestUser()

	usersModel.EXPECT().
		FindOne(gomock.Any(), user.Id).
		Return(nil, models.ErrNotFound)

	_, err := NewRefreshLogic(context.Background(), svcCtx).Refresh(&types.RefreshReq{
		RefreshToken: signRefreshToken(t, testRefreshSecret, testRefreshExpire, user),
	})
	if !errors.Is(err, errUnauthorized) {
		t.Errorf("err = %v, want errUnauthorized", err)
	}
}

func TestRefreshDisabledUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, usersModel := newTestSvcCtx(ctrl)
	user := newTestUser()
	user.DisabledAt = disabledAt()

	usersModel.EXPECT().
		FindOne(gomock.Any(), user.Id).
		Return(user, nil)

	_, err := NewRefreshLogic(context.Background(), svcCtx).Refresh(&types.RefreshReq{
		RefreshToken: signRefreshToken(t, testRefreshSecret, testRefreshExpire, user),
	})
	if !errors.Is(err, errUserDisabled) {
		t.Errorf("err = %v, want errUserDisabled", err)
	}
}

func TestRefreshFindError(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, usersModel := newTestSvcCtx(ctrl)
	user := newTestUser()

	dbErr := errors.New("db down")
	usersModel.EXPECT().
		FindOne(gomock.Any(), user.Id).
		Return(nil, dbErr)

	_, err := NewRefreshLogic(context.Background(), svcCtx).Refresh(&types.RefreshReq{
		RefreshToken: signRefreshToken(t, testRefreshSecret, testRefreshExpire, user),
	})
	if !errors.Is(err, dbErr) {
		t.Errorf("err = %v, want %v", err, dbErr)
	}
}
