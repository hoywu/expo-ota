// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListAppsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListAppsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAppsLogic {
	return &ListAppsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListAppsLogic) ListApps() (resp *types.ListAppsResp, err error) {
	apps, err := l.svcCtx.AppsModel.FindAllActive(l.ctx)
	if err != nil {
		return nil, err
	}

	items := make([]types.AppResp, 0, len(apps))
	for _, app := range apps {
		items = append(items, *appToResp(app))
	}

	return &types.ListAppsResp{Items: items}, nil
}

func appToResp(app *models.Apps) *types.AppResp {
	return &types.AppResp{
		Id:          app.Id,
		AppSlug:     app.AppSlug,
		Name:        app.Name,
		Description: app.Description.String,
		CreatedAt:   formatTime(app.CreatedAt),
	}
}
