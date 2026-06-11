// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateApiTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateApiTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateApiTokenLogic {
	return &CreateApiTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateApiTokenLogic) CreateApiToken(req *types.CreateTokenReq) (resp *types.CreateTokenResp, err error) {
	// todo: add your logic here and delete this line

	return
}
