// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"database/sql"
	"errors"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

type EnableUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEnableUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnableUserLogic {
	return &EnableUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EnableUserLogic) EnableUser(req *types.UserIdPath) (resp *types.ActionResp, err error) {
	user, err := l.svcCtx.UsersModel.FindOne(l.ctx, req.UserId)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errUserNotFound
		}
		return nil, err
	}

	if user.DisabledAt.Valid {
		user.DisabledAt = sql.NullTime{}
		if err := l.svcCtx.UsersModel.Update(l.ctx, user); err != nil {
			return nil, err
		}
	}

	writeAudit(l.ctx, l.svcCtx, "enable_user", "", "user", user.Id, nil)

	return &types.ActionResp{UserId: user.Id}, nil
}
