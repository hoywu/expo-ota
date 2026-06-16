// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListUsersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUsersLogic {
	return &ListUsersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListUsersLogic) ListUsers() (resp *types.ListUsersResp, err error) {
	users, err := l.svcCtx.UsersModel.FindAll(l.ctx)
	if err != nil {
		return nil, err
	}

	items := make([]types.UserItem, 0, len(users))
	for _, user := range users {
		items = append(items, userToItem(user))
	}

	return &types.ListUsersResp{Items: items}, nil
}
