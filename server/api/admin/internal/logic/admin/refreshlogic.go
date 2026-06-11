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
)

type RefreshLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshLogic {
	return &RefreshLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RefreshLogic) Refresh(req *types.RefreshReq) (resp *types.RefreshResp, err error) {
	claims, err := parseRefreshToken(l.svcCtx.Config.RefreshSecret, req.RefreshToken)
	if err != nil {
		return nil, err
	}

	userID, err := userIDFromClaims(claims)
	if err != nil {
		return nil, err
	}

	user, err := l.svcCtx.UsersModel.FindOne(l.ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errUnauthorized
		}
		return nil, err
	}

	if err := validateActiveUser(user); err != nil {
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

	return tokensToRefreshResp(accessToken, refreshToken, l.svcCtx.Config.Auth.AccessExpire), nil
}
