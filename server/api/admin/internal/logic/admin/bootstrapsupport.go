package admin

import (
	"context"
	"errors"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logc"
)

// BootstrapInitialAdmin creates the first admin user from the configured
// INITIAL_ADMIN_* credentials when the users table is empty (§11.1.3). It is a
// no-op once any user exists and never overwrites an existing account.
func BootstrapInitialAdmin(ctx context.Context, svcCtx *svc.ServiceContext) error {
	username := svcCtx.Config.InitialAdmin.Username
	password := svcCtx.Config.InitialAdmin.Password
	if username == "" || password == "" {
		return nil
	}

	users, err := svcCtx.UsersModel.FindAll(ctx)
	if err != nil {
		return err
	}
	if len(users) > 0 {
		return nil
	}

	if err := validatePassword(password); err != nil {
		return errors.New("INITIAL_ADMIN_PASSWORD does not meet the password policy (>=10 chars with letters and digits)")
	}

	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	id, err := newUUID()
	if err != nil {
		return err
	}
	if _, err := svcCtx.UsersModel.Insert(ctx, &models.Users{
		Id:           id,
		Username:     username,
		PasswordHash: hash,
	}); err != nil {
		return err
	}

	logc.Infof(ctx, "bootstrapped initial admin user %q", username)
	return nil
}
