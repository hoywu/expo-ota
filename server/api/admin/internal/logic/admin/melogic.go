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

type MeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MeLogic {
	return &MeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MeLogic) Me() (resp *types.MeResp, err error) {
	userID, err := userIDFromContext(l.ctx)
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

	return userToMeResp(user), nil
}
