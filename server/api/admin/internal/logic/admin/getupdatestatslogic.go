// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUpdateStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUpdateStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUpdateStatsLogic {
	return &GetUpdateStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUpdateStatsLogic) GetUpdateStats(req *types.UpdateIdPath) (resp *types.UpdateStatsResp, err error) {
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	update, err := findAppUpdate(l.ctx, l.svcCtx, app, req.UpdateId)
	if err != nil {
		return nil, err
	}

	if update.Status != "published" {
		return emptyUpdateStats(), nil
	}

	return buildUpdateStats(l.ctx, l.svcCtx, app, update)
}
