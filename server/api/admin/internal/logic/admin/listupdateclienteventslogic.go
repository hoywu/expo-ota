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

type ListUpdateClientEventsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListUpdateClientEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUpdateClientEventsLogic {
	return &ListUpdateClientEventsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListUpdateClientEventsLogic) ListUpdateClientEvents(req *types.UpdateIdPath) (resp *types.ListUpdateClientEventsResp, err error) {
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	update, err := findAppUpdate(l.ctx, l.svcCtx, app, req.UpdateId)
	if err != nil {
		return nil, err
	}

	if update.Status != "published" {
		return &types.ListUpdateClientEventsResp{Items: []types.ClientEventItem{}}, nil
	}

	rows, err := l.svcCtx.ClientEventsModel.ListByUpdate(l.ctx, app.Id, update.ManifestUuid)
	if err != nil {
		return nil, err
	}

	items := make([]types.ClientEventItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapClientEventItem(row))
	}

	return &types.ListUpdateClientEventsResp{Items: items}, nil
}

func mapClientEventItem(row *models.ClientEvents) types.ClientEventItem {
	item := types.ClientEventItem{
		EventId:    row.EventId,
		EventType:  row.EventType,
		OccurredAt: formatTime(row.OccurredAt),
		ReceivedAt: formatTime(row.ReceivedAt),
		DeviceId:   row.DeviceId,
	}
	if row.AppVersion.Valid {
		item.AppVersion = row.AppVersion.String
	}
	if row.OsVersion.Valid {
		item.OsVersion = row.OsVersion.String
	}
	if row.DurationMs.Valid {
		item.DurationMs = row.DurationMs.Int64
	}
	if row.ErrorCode.Valid {
		item.ErrorCode = row.ErrorCode.String
	}
	if row.ErrorMessage.Valid {
		item.ErrorMessage = row.ErrorMessage.String
	}
	if row.Platform.Valid {
		item.Platform = row.Platform.String
	}
	if row.RuntimeVersion.Valid {
		item.RuntimeVersion = row.RuntimeVersion.String
	}
	return item
}
