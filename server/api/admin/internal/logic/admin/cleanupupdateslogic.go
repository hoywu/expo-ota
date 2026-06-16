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

var errInvalidKeepLatestN = httperr.New(http.StatusBadRequest, "keepLatestN must be at least 1")

type CleanupUpdatesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCleanupUpdatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CleanupUpdatesLogic {
	return &CleanupUpdatesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CleanupUpdatesLogic) CleanupUpdates(req *types.CleanupReq) (resp *types.CleanupResp, err error) {
	if req.KeepLatestN < 1 {
		return nil, errInvalidKeepLatestN
	}

	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	deletedIds, err := l.svcCtx.UpdatesModel.SoftDeleteBeyondRank(l.ctx, app.Id, req.KeepLatestN)
	if err != nil {
		return nil, err
	}

	// Orphan asset GC (COS object + assets row removal) runs asynchronously
	// in the asset-gc job; here we only report how many became orphaned.
	orphanCount, err := l.svcCtx.AssetsModel.CountOrphanByAppId(l.ctx, app.Id)
	if err != nil {
		return nil, err
	}

	writeAudit(l.ctx, l.svcCtx, "cleanup_updates", app.Id, "app", app.Id, map[string]any{
		"keepLatestN":  req.KeepLatestN,
		"deletedCount": len(deletedIds),
	})

	if deletedIds == nil {
		deletedIds = []string{}
	}

	return &types.CleanupResp{
		DeletedUpdateIds: deletedIds,
		OrphanAssetCount: int(orphanCount),
	}, nil
}
