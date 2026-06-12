package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

const testPassword = "correct horse battery staple"

func newLoginUser(t *testing.T) *models.Users {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}

	user := newTestUser()
	user.PasswordHash = string(hash)

	return user
}

func TestLoginSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, usersModel := newTestSvcCtx(ctrl)
	user := newLoginUser(t)

	usersModel.EXPECT().
		FindOneByUsername(gomock.Any(), "alice").
		Return(user, nil)
	usersModel.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, updated *models.Users) error {
			if updated.Id != user.Id {
				t.Errorf("Update called with user %q, want %q", updated.Id, user.Id)
			}
			if !updated.LastLoginAt.Valid {
				t.Error("Update should set LastLoginAt")
			}
			return nil
		})

	resp, err := NewLoginLogic(context.Background(), svcCtx).Login(&types.LoginReq{
		Username: "alice",
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
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

func TestLoginUserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, usersModel := newTestSvcCtx(ctrl)

	usersModel.EXPECT().
		FindOneByUsername(gomock.Any(), "ghost").
		Return(nil, models.ErrNotFound)

	_, err := NewLoginLogic(context.Background(), svcCtx).Login(&types.LoginReq{
		Username: "ghost",
		Password: testPassword,
	})
	if !errors.Is(err, errInvalidCredentials) {
		t.Errorf("err = %v, want errInvalidCredentials", err)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, usersModel := newTestSvcCtx(ctrl)

	usersModel.EXPECT().
		FindOneByUsername(gomock.Any(), "alice").
		Return(newLoginUser(t), nil)

	_, err := NewLoginLogic(context.Background(), svcCtx).Login(&types.LoginReq{
		Username: "alice",
		Password: "wrong password",
	})
	if !errors.Is(err, errInvalidCredentials) {
		t.Errorf("err = %v, want errInvalidCredentials", err)
	}
}

func TestLoginDisabledUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, usersModel := newTestSvcCtx(ctrl)

	user := newLoginUser(t)
	user.DisabledAt = disabledAt()
	usersModel.EXPECT().
		FindOneByUsername(gomock.Any(), "alice").
		Return(user, nil)

	_, err := NewLoginLogic(context.Background(), svcCtx).Login(&types.LoginReq{
		Username: "alice",
		Password: testPassword,
	})
	if !errors.Is(err, errUserDisabled) {
		t.Errorf("err = %v, want errUserDisabled", err)
	}
}

func TestLoginFindError(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, usersModel := newTestSvcCtx(ctrl)

	dbErr := errors.New("db down")
	usersModel.EXPECT().
		FindOneByUsername(gomock.Any(), "alice").
		Return(nil, dbErr)

	_, err := NewLoginLogic(context.Background(), svcCtx).Login(&types.LoginReq{
		Username: "alice",
		Password: testPassword,
	})
	if !errors.Is(err, dbErr) {
		t.Errorf("err = %v, want %v", err, dbErr)
	}
}

func TestLoginUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, usersModel := newTestSvcCtx(ctrl)

	dbErr := errors.New("db down")
	usersModel.EXPECT().
		FindOneByUsername(gomock.Any(), "alice").
		Return(newLoginUser(t), nil)
	usersModel.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(dbErr)

	_, err := NewLoginLogic(context.Background(), svcCtx).Login(&types.LoginReq{
		Username: "alice",
		Password: testPassword,
	})
	if !errors.Is(err, dbErr) {
		t.Errorf("err = %v, want %v", err, dbErr)
	}
}
