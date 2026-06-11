// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

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
	// todo: add your logic here and delete this line

	return
}
