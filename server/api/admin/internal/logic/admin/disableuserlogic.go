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

type DisableUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDisableUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DisableUserLogic {
	return &DisableUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DisableUserLogic) DisableUser(req *types.UserIdPath) (resp *types.ActionResp, err error) {
	user, err := l.svcCtx.UsersModel.FindOne(l.ctx, req.UserId)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errUserNotFound
		}
		return nil, err
	}

	if !user.DisabledAt.Valid {
		user.DisabledAt = nullableNow()
		if err := l.svcCtx.UsersModel.Update(l.ctx, user); err != nil {
			return nil, err
		}
	}

	writeAudit(l.ctx, l.svcCtx, "disable_user", "", "user", user.Id, nil)

	return &types.ActionResp{UserId: user.Id}, nil
}
