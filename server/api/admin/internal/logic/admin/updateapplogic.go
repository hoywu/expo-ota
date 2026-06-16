// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAppLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAppLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAppLogic {
	return &UpdateAppLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateAppLogic) UpdateApp(req *types.UpdateAppReq) (resp *types.AppResp, err error) {
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	// appSlug is immutable (§2.1); only name/description may change.
	if req.Name != "" {
		app.Name = req.Name
	}
	if req.Description != "" {
		app.Description = nullString(req.Description)
	}

	if err := l.svcCtx.AppsModel.Update(l.ctx, app); err != nil {
		return nil, err
	}

	writeAudit(l.ctx, l.svcCtx, "update_app", app.Id, "app", app.Id, map[string]any{
		"name":        app.Name,
		"description": app.Description.String,
	})

	return appToResp(app), nil
}
