// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"net/http"

	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

var errUpdateNotPending = httperr.New(http.StatusConflict, "update is not in pending state")

type PublishUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPublishUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishUpdateLogic {
	return &PublishUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PublishUpdateLogic) PublishUpdate(req *types.UpdateIdPath) (resp *types.PublishResp, err error) {
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	update, err := findAppUpdate(l.ctx, l.svcCtx, app, req.UpdateId)
	if err != nil {
		return nil, err
	}

	affected, err := l.svcCtx.UpdatesModel.Publish(l.ctx, update.Id)
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, errUpdateNotPending
	}

	published, err := l.svcCtx.UpdatesModel.FindOne(l.ctx, update.Id)
	if err != nil {
		return nil, err
	}

	writeAudit(l.ctx, l.svcCtx, "publish_update", app.Id, "update", update.Id, map[string]any{
		"manifestUuid": published.ManifestUuid,
	})

	return &types.PublishResp{
		UpdateId:     published.Id,
		ManifestUuid: published.ManifestUuid,
		Status:       published.Status,
		PublishedAt:  formatNullTime(published.PublishedAt),
	}, nil
}
