package admin

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/hoywu/expo-ota/server/db/models"
	"go.uber.org/mock/gomock"
)

func TestMeSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, usersModel := newTestSvcCtx(ctrl)

	user := newTestUser()
	user.LastLoginAt = sql.NullTime{
		Time:  time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
		Valid: true,
	}
	usersModel.EXPECT().
		FindOne(gomock.Any(), user.Id).
		Return(user, nil)

	resp, err := NewMeLogic(ctxWithUserID(user.Id), svcCtx).Me()
	if err != nil {
		t.Fatalf("Me returned error: %v", err)
	}

	if resp.UserId != user.Id {
		t.Errorf("UserId = %q, want %q", resp.UserId, user.Id)
	}
	if resp.Username != user.Username {
		t.Errorf("Username = %q, want %q", resp.Username, user.Username)
	}
	if want := "2026-01-02T03:04:05Z"; resp.CreatedAt != want {
		t.Errorf("CreatedAt = %q, want %q", resp.CreatedAt, want)
	}
	if want := "2026-06-01T10:00:00Z"; resp.LastLoginAt != want {
		t.Errorf("LastLoginAt = %q, want %q", resp.LastLoginAt, want)
	}
}

func TestMeNeverLoggedIn(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, usersModel := newTestSvcCtx(ctrl)

	user := newTestUser()
	usersModel.EXPECT().
		FindOne(gomock.Any(), user.Id).
		Return(user, nil)

	resp, err := NewMeLogic(ctxWithUserID(user.Id), svcCtx).Me()
	if err != nil {
		t.Fatalf("Me returned error: %v", err)
	}

	if resp.LastLoginAt != "" {
		t.Errorf("LastLoginAt = %q, want empty", resp.LastLoginAt)
	}
}

func TestMeMissingUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, _ := newTestSvcCtx(ctrl)

	_, err := NewMeLogic(context.Background(), svcCtx).Me()
	if !errors.Is(err, errUnauthorized) {
		t.Errorf("err = %v, want errUnauthorized", err)
	}
}

func TestMeUserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, usersModel := newTestSvcCtx(ctrl)

	usersModel.EXPECT().
		FindOne(gomock.Any(), "user-1").
		Return(nil, models.ErrNotFound)

	_, err := NewMeLogic(ctxWithUserID("user-1"), svcCtx).Me()
	if !errors.Is(err, errUnauthorized) {
		t.Errorf("err = %v, want errUnauthorized", err)
	}
}

func TestMeDisabledUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, usersModel := newTestSvcCtx(ctrl)

	user := newTestUser()
	user.DisabledAt = disabledAt()
	usersModel.EXPECT().
		FindOne(gomock.Any(), user.Id).
		Return(user, nil)

	_, err := NewMeLogic(ctxWithUserID(user.Id), svcCtx).Me()
	if !errors.Is(err, errUserDisabled) {
		t.Errorf("err = %v, want errUserDisabled", err)
	}
}

func TestMeFindError(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, usersModel := newTestSvcCtx(ctrl)

	dbErr := errors.New("db down")
	usersModel.EXPECT().
		FindOne(gomock.Any(), "user-1").
		Return(nil, dbErr)

	_, err := NewMeLogic(ctxWithUserID("user-1"), svcCtx).Me()
	if !errors.Is(err, dbErr) {
		t.Errorf("err = %v, want %v", err, dbErr)
	}
}
