package admin

import (
	"context"
	"database/sql"
	"testing"

	"github.com/hoywu/expo-ota/server/db/models"
	"go.uber.org/mock/gomock"
)

func TestBootstrapInitialAdminCreatesWhenEmpty(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)
	svcCtx.Config.InitialAdmin.Username = "admin"
	svcCtx.Config.InitialAdmin.Password = "ChangeMeNow123"

	m.Users.EXPECT().FindAll(gomock.Any()).Return(nil, nil)
	var inserted *models.Users
	m.Users.EXPECT().Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, u *models.Users) (sql.Result, error) {
			inserted = u
			return nil, nil
		})

	if err := BootstrapInitialAdmin(context.Background(), svcCtx); err != nil {
		t.Fatalf("BootstrapInitialAdmin returned error: %v", err)
	}
	if inserted == nil {
		t.Fatal("expected a user to be inserted")
	}
	if inserted.Username != "admin" {
		t.Errorf("Username = %q, want admin", inserted.Username)
	}
	if inserted.PasswordHash == "" || inserted.PasswordHash == "ChangeMeNow123" {
		t.Errorf("PasswordHash = %q, want bcrypt hash", inserted.PasswordHash)
	}
}

func TestBootstrapInitialAdminSkipsWhenUsersExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)
	svcCtx.Config.InitialAdmin.Username = "admin"
	svcCtx.Config.InitialAdmin.Password = "ChangeMeNow123"

	m.Users.EXPECT().FindAll(gomock.Any()).Return([]*models.Users{{Id: "u1"}}, nil)
	// No Insert expectation: gomock fails the test if Insert is called.

	if err := BootstrapInitialAdmin(context.Background(), svcCtx); err != nil {
		t.Fatalf("BootstrapInitialAdmin returned error: %v", err)
	}
}

func TestBootstrapInitialAdminNoopWithoutCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, _ := newFullTestSvcCtx(ctrl)
	// InitialAdmin left empty: no DB access expected at all.

	if err := BootstrapInitialAdmin(context.Background(), svcCtx); err != nil {
		t.Fatalf("BootstrapInitialAdmin returned error: %v", err)
	}
}

func TestBootstrapInitialAdminRejectsWeakPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)
	svcCtx.Config.InitialAdmin.Username = "admin"
	svcCtx.Config.InitialAdmin.Password = "weak"

	m.Users.EXPECT().FindAll(gomock.Any()).Return(nil, nil)
	// No Insert expectation: weak password must abort before insert.

	if err := BootstrapInitialAdmin(context.Background(), svcCtx); err == nil {
		t.Fatal("expected error for weak password, got nil")
	}
}
