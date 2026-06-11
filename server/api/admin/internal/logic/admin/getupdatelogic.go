// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUpdateLogic {
	return &GetUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUpdateLogic) GetUpdate(req *types.UpdateIdPath) (resp *types.UpdateDetailResp, err error) {
	// todo: add your logic here and delete this line

	return
}
