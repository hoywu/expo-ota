package admin

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

func TestCreateUserNormalizesUsernameAndHashes(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Users.EXPECT().FindOneByUsername(gomock.Any(), "alice").Return(nil, models.ErrNotFound)
	var inserted *models.Users
	m.Users.EXPECT().Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, user *models.Users) (sql.Result, error) {
			inserted = user
			return nil, nil
		})
	m.Users.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id string) (*models.Users, error) {
			inserted.Id = id
			return inserted, nil
		})

	resp, err := NewCreateUserLogic(ctxWithUserID("user-1"), svcCtx).CreateUser(&types.CreateUserReq{
		Username: "  Alice ",
		Password: "passw0rd123",
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if resp.Username != "alice" {
		t.Errorf("Username = %q, want alice", resp.Username)
	}
	if bcrypt.CompareHashAndPassword([]byte(inserted.PasswordHash), []byte("passw0rd123")) != nil {
		t.Error("stored hash does not match password")
	}
}

func TestCreateUserWeakPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, _ := newFullTestSvcCtx(ctrl)

	for _, password := range []string{"short1a", "alllettersonly", "1234567890123"} {
		_, err := NewCreateUserLogic(ctxWithUserID("user-1"), svcCtx).CreateUser(&types.CreateUserReq{
			Username: "bob", Password: password,
		})
		if !errors.Is(err, errWeakPassword) {
			t.Errorf("password %q: err = %v, want errWeakPassword", password, err)
		}
	}
}

func TestChangePasswordUserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Users.EXPECT().FindOne(gomock.Any(), "user-2").Return(nil, models.ErrNotFound)

	_, err := NewChangePasswordLogic(ctxWithUserID("user-1"), svcCtx).ChangePassword(&types.ChangePasswordReq{
		UserId: "user-2", Password: "newpassw0rd1",
	})
	if !errors.Is(err, errUserNotFound) {
		t.Errorf("err = %v, want errUserNotFound", err)
	}
}

func TestDisableUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	user := newTestUser()
	m.Users.EXPECT().FindOne(gomock.Any(), user.Id).Return(user, nil)
	m.Users.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, u *models.Users) error {
			if !u.DisabledAt.Valid {
				t.Error("DisabledAt not set")
			}
			return nil
		})

	resp, err := NewDisableUserLogic(ctxWithUserID("user-9"), svcCtx).DisableUser(&types.UserIdPath{UserId: user.Id})
	if err != nil {
		t.Fatalf("DisableUser returned error: %v", err)
	}
	if resp.UserId != user.Id {
		t.Errorf("UserId = %q", resp.UserId)
	}
}

func TestEnableUserClearsDisabledAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	user := newTestUser()
	user.DisabledAt = disabledAt()
	m.Users.EXPECT().FindOne(gomock.Any(), user.Id).Return(user, nil)
	m.Users.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, u *models.Users) error {
			if u.DisabledAt.Valid {
				t.Error("DisabledAt still set")
			}
			return nil
		})

	if _, err := NewEnableUserLogic(ctxWithUserID("user-9"), svcCtx).EnableUser(&types.UserIdPath{UserId: user.Id}); err != nil {
		t.Fatalf("EnableUser returned error: %v", err)
	}
}
