// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"errors"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListUpdatesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListUpdatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUpdatesLogic {
	return &ListUpdatesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListUpdatesLogic) ListUpdates(req *types.ListUpdatesReq) (resp *types.ListUpdatesResp, err error) {
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	filter := models.UpdatesFilter{
		Platform: req.Platform,
		Status:   req.Status,
		CursorId: req.Cursor,
		Limit:    req.Limit,
	}
	if req.RuntimeVersion != "" {
		rv, err := l.svcCtx.RuntimeVersionsModel.FindOneByAppIdVersion(l.ctx, app.Id, req.RuntimeVersion)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return &types.ListUpdatesResp{Items: []types.UpdateListItem{}}, nil
			}
			return nil, err
		}
		filter.RuntimeVersionId = rv.Id
	}

	updates, err := l.svcCtx.UpdatesModel.FindManyByAppId(l.ctx, app.Id, filter)
	if err != nil {
		return nil, err
	}

	// Resolve runtime version strings (few distinct ids per page).
	versions := map[string]string{}
	items := make([]types.UpdateListItem, 0, len(updates))
	for _, update := range updates {
		version, ok := versions[update.RuntimeVersionId]
		if !ok {
			rv, err := l.svcCtx.RuntimeVersionsModel.FindOne(l.ctx, update.RuntimeVersionId)
			if err != nil {
				return nil, err
			}
			version = rv.Version
			versions[update.RuntimeVersionId] = version
		}

		items = append(items, types.UpdateListItem{
			Id:             update.Id,
			RuntimeVersion: version,
			Platform:       update.Platform,
			ManifestUuid:   update.ManifestUuid,
			Status:         update.Status,
			Message:        update.Message.String,
			CreatedAt:      formatTime(update.CreatedAt),
			PublishedAt:    formatNullTime(update.PublishedAt),
		})
	}

	nextCursor := ""
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if len(updates) == limit {
		nextCursor = updates[len(updates)-1].Id
	}

	return &types.ListUpdatesResp{Items: items, NextCursor: nextCursor}, nil
}
