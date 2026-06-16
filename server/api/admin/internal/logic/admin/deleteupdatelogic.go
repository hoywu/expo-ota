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

// minPublishedRankForDelete: a published update may only be deleted when it
// lags at least 3 versions behind in its stream (§9.2).
const minPublishedRankForDelete = 3

var errUpdateTooRecent = httperr.New(http.StatusBadRequest,
	"update must be at least 3 published versions behind in its stream before deletion")

type DeleteUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteUpdateLogic {
	return &DeleteUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteUpdateLogic) DeleteUpdate(req *types.UpdateIdPath) (resp *types.EmptyResp, err error) {
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	update, err := findAppUpdate(l.ctx, l.svcCtx, app, req.UpdateId)
	if err != nil {
		return nil, err
	}

	// Pending drafts can always be deleted; published updates must lag
	// behind by more than minPublishedRankForDelete versions.
	if update.Status == "published" {
		rank, err := l.svcCtx.UpdatesModel.PublishedRank(l.ctx, update.Id)
		if err != nil {
			return nil, err
		}
		if rank <= minPublishedRankForDelete {
			return nil, errUpdateTooRecent
		}
	}

	if err := l.svcCtx.UpdatesModel.SoftDelete(l.ctx, update.Id); err != nil {
		return nil, err
	}

	// Orphan asset GC (COS object + assets row removal) runs asynchronously
	// in the asset-gc job; the admin API only soft-deletes.
	writeAudit(l.ctx, l.svcCtx, "delete_update", app.Id, "update", update.Id, map[string]any{
		"manifestUuid": update.ManifestUuid,
	})

	return &types.EmptyResp{}, nil
}
