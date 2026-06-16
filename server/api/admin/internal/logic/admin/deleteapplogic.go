// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteAppLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteAppLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAppLogic {
	return &DeleteAppLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteAppLogic) DeleteApp(req *types.AppSlugPath) (resp *types.EmptyResp, err error) {
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	app.DeletedAt = nullableNow()
	if err := l.svcCtx.AppsModel.Update(l.ctx, app); err != nil {
		return nil, err
	}

	writeAudit(l.ctx, l.svcCtx, "delete_app", app.Id, "app", app.Id, map[string]any{
		"appSlug": app.AppSlug,
	})

	return &types.EmptyResp{}, nil
}
