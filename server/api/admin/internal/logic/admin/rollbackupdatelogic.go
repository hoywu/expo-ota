// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

type RollbackUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRollbackUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RollbackUpdateLogic {
	return &RollbackUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// RollbackUpdate implements "republish previous" (§9.1): it copies the source
// update into a new pending draft that reuses the same assets.
func (l *RollbackUpdateLogic) RollbackUpdate(req *types.UpdateIdPath) (resp *types.RollbackResp, err error) {
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	source, err := findAppUpdate(l.ctx, l.svcCtx, app, req.UpdateId)
	if err != nil {
		return nil, err
	}

	rv, err := l.svcCtx.RuntimeVersionsModel.FindOne(l.ctx, source.RuntimeVersionId)
	if err != nil {
		return nil, err
	}

	sourceAssets, err := l.svcCtx.UpdateAssetsModel.FindAllByUpdateId(l.ctx, source.Id)
	if err != nil {
		return nil, err
	}
	assetIds := make([]string, 0, len(sourceAssets))
	for _, ua := range sourceAssets {
		assetIds = append(assetIds, ua.AssetId)
	}
	assets, err := l.svcCtx.AssetsModel.FindAllByIds(l.ctx, assetIds)
	if err != nil {
		return nil, err
	}
	assetById := make(map[string]*models.Assets, len(assets))
	for _, asset := range assets {
		assetById[asset.Id] = asset
	}

	// Rebuild the manifest with a fresh createdAt; assets (and their URLs)
	// are reused verbatim.
	createdAt := time.Now()
	var launchAsset manifestAsset
	manifestAssets := []manifestAsset{}
	for _, ua := range sourceAssets {
		asset, ok := assetById[ua.AssetId]
		if !ok {
			continue
		}
		entry := assetToManifestAsset(ua.AssetKey, asset, l.svcCtx.Store.PublicURL(asset.StorageKey))
		if ua.AssetId == source.LaunchAssetId {
			launchAsset = entry
		} else {
			manifestAssets = append(manifestAssets, entry)
		}
	}

	var metadata, expoConfig map[string]any
	_ = json.Unmarshal([]byte(source.ManifestMetadata), &metadata)
	if source.ExpoConfig.Valid {
		_ = json.Unmarshal([]byte(source.ExpoConfig.String), &expoConfig)
	}

	manifest := buildManifest(rv.Version, createdAt, launchAsset, manifestAssets, metadata, expoConfig)
	manifestUuid, err := computeManifestUuid(manifest)
	if err != nil {
		return nil, err
	}
	manifest["id"] = manifestUuid

	snapshot, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}

	newId, err := newUUID()
	if err != nil {
		return nil, err
	}

	newUpdate := &models.Updates{
		Id:               newId,
		AppId:            source.AppId,
		RuntimeVersionId: source.RuntimeVersionId,
		Platform:         source.Platform,
		ManifestUuid:     manifestUuid,
		LaunchAssetId:    source.LaunchAssetId,
		Status:           "pending",
		Message:          source.Message,
		GitCommitHash:    source.GitCommitHash,
		ManifestMetadata: source.ManifestMetadata,
		Extra:            source.Extra,
		ExpoConfig:       source.ExpoConfig,
		ManifestSnapshot: string(snapshot),
		RolledBackFrom:   sql.NullString{String: source.Id, Valid: true},
	}

	newAssetRows := make([]*models.UpdateAssets, 0, len(sourceAssets))
	for _, ua := range sourceAssets {
		rowId, err := newUUID()
		if err != nil {
			return nil, err
		}
		newAssetRows = append(newAssetRows, &models.UpdateAssets{
			Id:        rowId,
			UpdateId:  newId,
			AssetId:   ua.AssetId,
			AssetKey:  ua.AssetKey,
			FileExt:   ua.FileExt,
			SortOrder: ua.SortOrder,
		})
	}

	if err := l.svcCtx.UpdatesModel.InsertWithAssets(l.ctx, newUpdate, newAssetRows); err != nil {
		return nil, err
	}

	created, err := l.svcCtx.UpdatesModel.FindOne(l.ctx, newId)
	if err != nil {
		return nil, err
	}

	writeAudit(l.ctx, l.svcCtx, "rollback_update", app.Id, "update", newId, map[string]any{
		"rolledBackFrom": source.Id,
		"manifestUuid":   manifestUuid,
	})

	return &types.RollbackResp{
		UpdateId:     created.Id,
		ManifestUuid: created.ManifestUuid,
		CreatedAt:    formatTime(created.CreatedAt),
	}, nil
}
