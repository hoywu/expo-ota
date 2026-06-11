// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlanUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlanUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlanUploadLogic {
	return &PlanUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PlanUploadLogic) PlanUpload(req *types.PlanReq) (resp *types.PlanResp, err error) {
	// todo: add your logic here and delete this line

	return
}
