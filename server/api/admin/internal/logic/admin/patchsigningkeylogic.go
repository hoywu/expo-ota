// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PatchSigningKeyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPatchSigningKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PatchSigningKeyLogic {
	return &PatchSigningKeyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PatchSigningKeyLogic) PatchSigningKey(req *types.PatchSigningKeyReq) (resp *types.SigningKeyResp, err error) {
	// todo: add your logic here and delete this line

	return
}
