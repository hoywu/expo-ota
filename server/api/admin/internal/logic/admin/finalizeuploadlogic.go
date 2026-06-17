// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
	"github.com/hoywu/expo-ota/server/internal/storage"

	"github.com/zeromicro/go-zero/core/logx"
)

var errNoLaunchAsset = httperr.New(http.StatusBadRequest,
	"assets must contain exactly one launch asset (contentType application/javascript)")

func errUpdateAlreadyFinalized(updateId string) error {
	return httperr.New(http.StatusConflict, fmt.Sprintf("update already finalized: %s", updateId))
}

type FinalizeUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFinalizeUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FinalizeUploadLogic {
	return &FinalizeUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FinalizeUploadLogic) FinalizeUpload(req *types.FinalizeReq) (resp *types.FinalizeResp, err error) {
	if len(req.Assets) == 0 {
		return nil, errNoAssets
	}

	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	launchKey, err := findLaunchAssetKey(req.Assets)
	if err != nil {
		return nil, err
	}

	// 1. Verify every asset exists in COS with the declared size.
	if err := l.verifyAssetsUploaded(app.AppSlug, req.Assets); err != nil {
		return nil, err
	}

	// 2. Upsert assets rows and resolve their ids.
	assetRowsByKey, err := l.upsertAssets(app, req.Assets)
	if err != nil {
		return nil, err
	}

	// 3. Lazily create the runtime version.
	runtimeVersion, err := l.findOrCreateRuntimeVersion(app.Id, req.RuntimeVersion)
	if err != nil {
		return nil, err
	}

	// 4-5. Build the manifest, compute the persistent manifest UUID, and
	// cache the snapshot.
	createdAt := time.Now()
	launchAssetRow := assetRowsByKey[launchKey]
	launchAsset := assetToManifestAsset(launchKey, launchAssetRow, l.svcCtx.Store.PublicURL(launchAssetRow.StorageKey))

	manifestAssets := []manifestAsset{}
	for _, asset := range req.Assets {
		if asset.Key == launchKey {
			continue
		}
		row := assetRowsByKey[asset.Key]
		manifestAssets = append(manifestAssets, assetToManifestAsset(asset.Key, row, l.svcCtx.Store.PublicURL(row.StorageKey)))
	}

	manifest := buildManifest(req.RuntimeVersion, createdAt, launchAsset, manifestAssets, req.ManifestMetadata, req.ExpoConfig)
	manifestUuid, err := computeFinalizeManifestUuid(manifest)
	if err != nil {
		return nil, err
	}
	manifest["id"] = manifestUuid

	snapshot, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}

	if existing, err := l.svcCtx.UpdatesModel.FindOneByAppIdManifestUuid(l.ctx, app.Id, manifestUuid); err == nil {
		return nil, errUpdateAlreadyFinalized(existing.Id)
	} else if !errors.Is(err, models.ErrNotFound) {
		return nil, err
	}

	// 6-7. Insert the pending update and its asset rows atomically.
	updateId, err := newUUID()
	if err != nil {
		return nil, err
	}

	update := &models.Updates{
		Id:               updateId,
		AppId:            app.Id,
		RuntimeVersionId: runtimeVersion.Id,
		Platform:         req.Platform,
		ManifestUuid:     manifestUuid,
		LaunchAssetId:    launchAssetRow.Id,
		Status:           "pending",
		Message:          nullString(req.Message),
		GitCommitHash:    nullString(req.GitCommitHash),
		ManifestMetadata: marshalOrEmptyObject(req.ManifestMetadata),
		Extra:            "{}",
		ExpoConfig:       nullString(marshalOrEmptyObject(req.ExpoConfig)),
		ManifestSnapshot: string(snapshot),
	}

	updateAssetRows := make([]*models.UpdateAssets, 0, len(req.Assets))
	for i, asset := range req.Assets {
		rowId, err := newUUID()
		if err != nil {
			return nil, err
		}
		updateAssetRows = append(updateAssetRows, &models.UpdateAssets{
			Id:        rowId,
			UpdateId:  updateId,
			AssetId:   assetRowsByKey[asset.Key].Id,
			AssetKey:  asset.Key,
			FileExt:   nullString(asset.FileExt),
			SortOrder: int64(i),
		})
	}

	if err := l.svcCtx.UpdatesModel.InsertWithAssets(l.ctx, update, updateAssetRows); err != nil {
		if existing, ferr := l.svcCtx.UpdatesModel.FindOneByAppIdManifestUuid(l.ctx, app.Id, manifestUuid); ferr == nil {
			return nil, errUpdateAlreadyFinalized(existing.Id)
		}
		return nil, err
	}

	created, err := l.svcCtx.UpdatesModel.FindOne(l.ctx, updateId)
	if err != nil {
		return nil, err
	}

	// 8. Audit.
	writeAudit(l.ctx, l.svcCtx, "finalize_update", app.Id, "update", updateId, map[string]any{
		"runtimeVersion": req.RuntimeVersion,
		"platform":       req.Platform,
		"manifestUuid":   manifestUuid,
		"assetCount":     len(req.Assets),
	})

	// 9. Return the pending draft; publish is a separate manual step.
	return &types.FinalizeResp{
		UpdateId:     created.Id,
		ManifestUuid: created.ManifestUuid,
		Status:       created.Status,
		CreatedAt:    formatTime(created.CreatedAt),
	}, nil
}

// findLaunchAssetKey locates the launch asset (the JS bundle) in the
// declared asset list.
func findLaunchAssetKey(assets []types.PlanAssetItem) (string, error) {
	launchKey := ""
	for _, asset := range assets {
		if asset.ContentType == "application/javascript" {
			if launchKey != "" {
				return "", errNoLaunchAsset
			}
			launchKey = asset.Key
		}
	}
	if launchKey == "" {
		return "", errNoLaunchAsset
	}
	return launchKey, nil
}

// verifyAssetsUploaded HEADs every declared asset in COS and reports the
// keys that are missing or have a mismatched size.
func (l *FinalizeUploadLogic) verifyAssetsUploaded(appSlug string, assets []types.PlanAssetItem) error {
	var badKeys []string
	for _, asset := range assets {
		if _, err := decodeSha256B64url(asset.Sha256); err != nil {
			return err
		}
		size, err := l.svcCtx.Store.Head(l.ctx, storage.AssetStorageKey(appSlug, asset.Sha256))
		if err != nil || size != asset.Size {
			badKeys = append(badKeys, asset.Key)
		}
	}
	if len(badKeys) > 0 {
		return httperr.New(http.StatusBadRequest,
			fmt.Sprintf("assets not uploaded or size mismatch: %s", strings.Join(badKeys, ", ")))
	}
	return nil
}

// upsertAssets inserts missing assets rows (idempotently) and returns the
// persisted row for each manifest asset key.
func (l *FinalizeUploadLogic) upsertAssets(app *models.Apps, assets []types.PlanAssetItem) (map[string]*models.Assets, error) {
	rows := make(map[string]*models.Assets, len(assets))
	for _, asset := range assets {
		rawSha, err := decodeSha256B64url(asset.Sha256)
		if err != nil {
			return nil, err
		}

		id, err := newUUID()
		if err != nil {
			return nil, err
		}
		err = l.svcCtx.AssetsModel.InsertIgnoreConflict(l.ctx, &models.Assets{
			Id:           id,
			AppId:        app.Id,
			Sha256:       models.ByteaHex(rawSha),
			Sha256B64url: asset.Sha256,
			SizeBytes:    asset.Size,
			ContentType:  asset.ContentType,
			FileExt:      nullString(asset.FileExt),
			StorageKey:   storage.AssetStorageKey(app.AppSlug, asset.Sha256),
		})
		if err != nil {
			return nil, err
		}

		row, err := l.svcCtx.AssetsModel.FindOneByAppIdSha256(l.ctx, app.Id, models.ByteaHex(rawSha))
		if err != nil {
			return nil, err
		}
		rows[asset.Key] = row
	}
	return rows, nil
}

func (l *FinalizeUploadLogic) findOrCreateRuntimeVersion(appId, version string) (*models.RuntimeVersions, error) {
	rv, err := l.svcCtx.RuntimeVersionsModel.FindOneByAppIdVersion(l.ctx, appId, version)
	if err == nil {
		return rv, nil
	}
	if !errors.Is(err, models.ErrNotFound) {
		return nil, err
	}

	id, err := newUUID()
	if err != nil {
		return nil, err
	}
	if _, err := l.svcCtx.RuntimeVersionsModel.Insert(l.ctx, &models.RuntimeVersions{
		Id:      id,
		AppId:   appId,
		Version: version,
	}); err != nil {
		return nil, err
	}
	return l.svcCtx.RuntimeVersionsModel.FindOne(l.ctx, id)
}

func marshalOrEmptyObject(m map[string]any) string {
	if m == nil {
		return "{}"
	}
	data, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(data)
}
