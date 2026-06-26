// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"encoding/json"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUpdateLogic {
	return &GetUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUpdateLogic) GetUpdate(req *types.UpdateIdPath) (resp *types.UpdateDetailResp, err error) {
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	update, err := findAppUpdate(l.ctx, l.svcCtx, app, req.UpdateId)
	if err != nil {
		return nil, err
	}

	rv, err := l.svcCtx.RuntimeVersionsModel.FindOne(l.ctx, update.RuntimeVersionId)
	if err != nil {
		return nil, err
	}

	launchAsset, err := l.svcCtx.AssetsModel.FindOne(l.ctx, update.LaunchAssetId)
	if err != nil {
		return nil, err
	}

	updateAssets, err := l.svcCtx.UpdateAssetsModel.FindAllByUpdateId(l.ctx, update.Id)
	if err != nil {
		return nil, err
	}

	assetIds := make([]string, 0, len(updateAssets))
	for _, ua := range updateAssets {
		assetIds = append(assetIds, ua.AssetId)
	}
	assets, err := l.svcCtx.AssetsModel.FindAllByIds(l.ctx, assetIds)
	if err != nil {
		return nil, err
	}
	assetById := make(map[string]int, len(assets))
	for i, asset := range assets {
		assetById[asset.Id] = i
	}

	launchAssetKey := ""
	items := make([]types.UpdateAssetItem, 0, len(updateAssets))
	for _, ua := range updateAssets {
		idx, ok := assetById[ua.AssetId]
		if !ok {
			continue
		}
		asset := assets[idx]
		if asset.Id == update.LaunchAssetId {
			launchAssetKey = ua.AssetKey
		}
		items = append(items, types.UpdateAssetItem{
			Key:     ua.AssetKey,
			Sha256:  asset.Sha256B64url,
			Size:    asset.SizeBytes,
			Url:     l.svcCtx.Store.PublicURL(asset.StorageKey),
			FileExt: ua.FileExt.String,
		})
	}

	var manifestPreview map[string]interface{}
	if update.ManifestSnapshot != "" {
		_ = json.Unmarshal([]byte(update.ManifestSnapshot), &manifestPreview)
	}

	stats, err := buildUpdateStats(l.ctx, l.svcCtx, app, update)
	if err != nil {
		return nil, err
	}

	return &types.UpdateDetailResp{
		Id:              update.Id,
		AppSlug:         app.AppSlug,
		RuntimeVersion:  rv.Version,
		Platform:        update.Platform,
		ManifestUuid:    update.ManifestUuid,
		Status:          update.Status,
		Message:         update.Message.String,
		GitCommitHash:   update.GitCommitHash.String,
		CreatedAt:       formatTime(update.CreatedAt),
		PublishedAt:     formatNullTime(update.PublishedAt),
		LaunchAssetKey:  launchAssetKey,
		LaunchAssetUrl:  l.svcCtx.Store.PublicURL(launchAsset.StorageKey),
		Assets:          items,
		ManifestPreview: manifestPreview,
		Stats:           *stats,
	}, nil
}
