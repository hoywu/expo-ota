// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSigningKeyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSigningKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSigningKeyLogic {
	return &GetSigningKeyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSigningKeyLogic) GetSigningKey(req *types.AppSlugPath) (resp *types.SigningKeyResp, err error) {
	// todo: add your logic here and delete this line

	return
}
