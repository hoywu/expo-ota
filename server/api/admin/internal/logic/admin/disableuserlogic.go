// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

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
	// todo: add your logic here and delete this line

	return
}
