// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"errors"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginReq) (resp *types.LoginResp, err error) {
	user, err := l.svcCtx.UsersModel.FindOneByUsername(l.ctx, req.Username)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			writeAudit(l.ctx, l.svcCtx, "login_failed", "", "user", "", map[string]any{"username": req.Username})
			return nil, errInvalidCredentials
		}
		return nil, err
	}

	if user.DisabledAt.Valid {
		return nil, errUserDisabled
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		writeAudit(l.ctx, l.svcCtx, "login_failed", "", "user", user.Id, map[string]any{"username": req.Username})
		return nil, errInvalidCredentials
	}

	user.LastLoginAt = nullableNow()
	if err := l.svcCtx.UsersModel.Update(l.ctx, user); err != nil {
		return nil, err
	}

	accessToken, err := newAccessToken(l.svcCtx.Config.Auth.AccessSecret, l.svcCtx.Config.Auth.AccessExpire, user)
	if err != nil {
		return nil, err
	}

	refreshToken, err := newRefreshToken(l.svcCtx.Config.RefreshSecret, l.svcCtx.Config.RefreshExpire, user)
	if err != nil {
		return nil, err
	}

	return tokensToLoginResp(accessToken, refreshToken, l.svcCtx.Config.Auth.AccessExpire), nil
}
