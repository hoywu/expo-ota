// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GenerateSigningKeyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGenerateSigningKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateSigningKeyLogic {
	return &GenerateSigningKeyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GenerateSigningKeyLogic) GenerateSigningKey(req *types.GenerateSigningKeyReq) (resp *types.SigningKeyResp, err error) {
	// todo: add your logic here and delete this line

	return
}
