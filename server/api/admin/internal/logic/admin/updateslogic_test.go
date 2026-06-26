package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
	"go.uber.org/mock/gomock"
)

func newTestUpdate() *models.Updates {
	return &models.Updates{
		Id:               "update-1",
		AppId:            "app-1",
		RuntimeVersionId: "rv-1",
		Platform:         "ios",
		ManifestUuid:     "11111111-2222-3333-4444-555555555555",
		LaunchAssetId:    "asset-launch",
		Status:           "published",
		ManifestMetadata: "{}",
		Extra:            "{}",
		ManifestSnapshot: "{}",
		CreatedAt:        time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		PublishedAt:      sql.NullTime{Time: time.Date(2026, 5, 1, 1, 0, 0, 0, time.UTC), Valid: true},
	}
}

func TestPublishUpdateSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	pending := newTestUpdate()
	pending.Status = "pending"
	pending.PublishedAt = sql.NullTime{}

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.Updates.EXPECT().FindOne(gomock.Any(), "update-1").Return(pending, nil)
	m.Updates.EXPECT().Publish(gomock.Any(), "update-1").Return(int64(1), nil)
	m.Updates.EXPECT().FindOne(gomock.Any(), "update-1").Return(newTestUpdate(), nil)

	resp, err := NewPublishUpdateLogic(ctxWithUserID("user-1"), svcCtx).PublishUpdate(&types.UpdateIdPath{
		AppSlug: "my-app", UpdateId: "update-1",
	})
	if err != nil {
		t.Fatalf("PublishUpdate returned error: %v", err)
	}
	if resp.Status != "published" || resp.PublishedAt == "" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestPublishUpdateNotPending(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.Updates.EXPECT().FindOne(gomock.Any(), "update-1").Return(newTestUpdate(), nil)
	m.Updates.EXPECT().Publish(gomock.Any(), "update-1").Return(int64(0), nil)

	_, err := NewPublishUpdateLogic(ctxWithUserID("user-1"), svcCtx).PublishUpdate(&types.UpdateIdPath{
		AppSlug: "my-app", UpdateId: "update-1",
	})
	if !errors.Is(err, errUpdateNotPending) {
		t.Errorf("err = %v, want errUpdateNotPending", err)
	}
}

func TestDeleteUpdateRecentPublishedRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.Updates.EXPECT().FindOne(gomock.Any(), "update-1").Return(newTestUpdate(), nil)
	m.Updates.EXPECT().PublishedRank(gomock.Any(), "update-1").Return(int64(2), nil)

	_, err := NewDeleteUpdateLogic(ctxWithUserID("user-1"), svcCtx).DeleteUpdate(&types.UpdateIdPath{
		AppSlug: "my-app", UpdateId: "update-1",
	})
	if !errors.Is(err, errUpdateTooRecent) {
		t.Errorf("err = %v, want errUpdateTooRecent", err)
	}
}

func TestDeleteUpdateOldPublishedSoftDeletes(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.Updates.EXPECT().FindOne(gomock.Any(), "update-1").Return(newTestUpdate(), nil)
	m.Updates.EXPECT().PublishedRank(gomock.Any(), "update-1").Return(int64(4), nil)
	m.Updates.EXPECT().SoftDelete(gomock.Any(), "update-1").Return(nil)

	if _, err := NewDeleteUpdateLogic(ctxWithUserID("user-1"), svcCtx).DeleteUpdate(&types.UpdateIdPath{
		AppSlug: "my-app", UpdateId: "update-1",
	}); err != nil {
		t.Fatalf("DeleteUpdate returned error: %v", err)
	}
}

func TestDeleteUpdatePendingAlwaysAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	pending := newTestUpdate()
	pending.Status = "pending"
	pending.PublishedAt = sql.NullTime{}

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.Updates.EXPECT().FindOne(gomock.Any(), "update-1").Return(pending, nil)
	m.Updates.EXPECT().SoftDelete(gomock.Any(), "update-1").Return(nil)

	if _, err := NewDeleteUpdateLogic(ctxWithUserID("user-1"), svcCtx).DeleteUpdate(&types.UpdateIdPath{
		AppSlug: "my-app", UpdateId: "update-1",
	}); err != nil {
		t.Fatalf("DeleteUpdate returned error: %v", err)
	}
}

func TestCleanupUpdatesValidatesKeepLatestN(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, _ := newFullTestSvcCtx(ctrl)

	_, err := NewCleanupUpdatesLogic(ctxWithUserID("user-1"), svcCtx).CleanupUpdates(&types.CleanupReq{
		AppSlug: "my-app", KeepLatestN: 0,
	})
	if !errors.Is(err, errInvalidKeepLatestN) {
		t.Errorf("err = %v, want errInvalidKeepLatestN", err)
	}
}

func TestCleanupUpdatesReportsDeletedAndOrphans(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.Updates.EXPECT().SoftDeleteBeyondRank(gomock.Any(), "app-1", 3).Return([]string{"u-4", "u-5"}, nil)
	m.Assets.EXPECT().CountOrphanByAppId(gomock.Any(), "app-1").Return(int64(7), nil)

	resp, err := NewCleanupUpdatesLogic(ctxWithUserID("user-1"), svcCtx).CleanupUpdates(&types.CleanupReq{
		AppSlug: "my-app", KeepLatestN: 3,
	})
	if err != nil {
		t.Fatalf("CleanupUpdates returned error: %v", err)
	}
	if len(resp.DeletedUpdateIds) != 2 || resp.OrphanAssetCount != 7 {
		t.Errorf("resp = %+v", resp)
	}
}

func TestGetUpdateIncludesRequestStats(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	update := newTestUpdate()
	launchAsset := &models.Assets{
		Id: "asset-launch", AppId: "app-1",
		Sha256B64url: "launchsha", ContentType: "application/javascript",
		StorageKey: "apps/my-app/assets/launchsha",
	}

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.Updates.EXPECT().FindOne(gomock.Any(), "update-1").Return(update, nil)
	m.RuntimeVersions.EXPECT().FindOne(gomock.Any(), "rv-1").
		Return(&models.RuntimeVersions{Id: "rv-1", AppId: "app-1", Version: "1.0.0"}, nil)
	m.Assets.EXPECT().FindOne(gomock.Any(), "asset-launch").Return(launchAsset, nil)
	m.UpdateAssets.EXPECT().FindAllByUpdateId(gomock.Any(), "update-1").Return([]*models.UpdateAssets{
		{Id: "ua-1", UpdateId: "update-1", AssetId: "asset-launch", AssetKey: "bundlekey", SortOrder: 0},
	}, nil)
	m.Assets.EXPECT().FindAllByIds(gomock.Any(), []string{"asset-launch"}).Return([]*models.Assets{launchAsset}, nil)
	m.ManifestRequests.EXPECT().CountDistinctDevices(gomock.Any(), "app-1", "update-1").Return(int64(5), nil)
	m.ManifestRequests.EXPECT().CountWithoutDeviceId(gomock.Any(), "app-1", "update-1").Return(int64(2), nil)
	m.ClientEvents.EXPECT().StatsByUpdate(gomock.Any(), "app-1", "11111111-2222-3333-4444-555555555555").Return(&models.ClientEventUpdateStats{
		SucceededDevices: 4,
		FailedDevices:    1,
	}, nil)

	resp, err := NewGetUpdateLogic(context.Background(), svcCtx).GetUpdate(&types.UpdateIdPath{
		AppSlug: "my-app", UpdateId: "update-1",
	})
	if err != nil {
		t.Fatalf("GetUpdate returned error: %v", err)
	}
	if resp.Stats.RequestedDevices != 5 || resp.Stats.RequestsWithoutDeviceId != 2 {
		t.Errorf("stats = %+v", resp.Stats)
	}
}

func TestGetUpdateStatsReturnsAggregatedCounts(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	update := newTestUpdate()

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.Updates.EXPECT().FindOne(gomock.Any(), "update-1").Return(update, nil)
	m.ManifestRequests.EXPECT().CountDistinctDevices(gomock.Any(), "app-1", "update-1").Return(int64(5), nil)
	m.ManifestRequests.EXPECT().CountWithoutDeviceId(gomock.Any(), "app-1", "update-1").Return(int64(2), nil)
	m.ClientEvents.EXPECT().StatsByUpdate(gomock.Any(), "app-1", "11111111-2222-3333-4444-555555555555").Return(&models.ClientEventUpdateStats{
		SucceededDevices: 4,
		FailedDevices:    1,
		DurationMinMs:    sql.NullInt64{Int64: 100, Valid: true},
		DurationMaxMs:    sql.NullInt64{Int64: 500, Valid: true},
		DurationAvgMs:    sql.NullInt64{Int64: 250, Valid: true},
	}, nil)

	resp, err := NewGetUpdateStatsLogic(context.Background(), svcCtx).GetUpdateStats(&types.UpdateIdPath{
		AppSlug: "my-app", UpdateId: "update-1",
	})
	if err != nil {
		t.Fatalf("GetUpdateStats returned error: %v", err)
	}
	if resp.RequestedDevices != 5 || resp.RequestsWithoutDeviceId != 2 {
		t.Errorf("stats = %+v", resp)
	}
	if resp.SucceededDevices != 4 || resp.FailedDevices != 1 {
		t.Errorf("event stats = %+v", resp)
	}
	if resp.DurationMinMs != 100 || resp.DurationMaxMs != 500 || resp.DurationAvgMs != 250 {
		t.Errorf("duration stats = %+v", resp)
	}
}

func TestGetUpdateStatsReturnsEmptyForPendingUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	update := newTestUpdate()
	update.Status = "pending"
	update.PublishedAt = sql.NullTime{}

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.Updates.EXPECT().FindOne(gomock.Any(), "update-1").Return(update, nil)

	resp, err := NewGetUpdateStatsLogic(context.Background(), svcCtx).GetUpdateStats(&types.UpdateIdPath{
		AppSlug: "my-app", UpdateId: "update-1",
	})
	if err != nil {
		t.Fatalf("GetUpdateStats returned error: %v", err)
	}
	if resp.RequestedDevices != 0 || resp.RequestsWithoutDeviceId != 0 ||
		resp.SucceededDevices != 0 || resp.FailedDevices != 0 {
		t.Errorf("stats = %+v", resp)
	}
}

func TestRollbackUpdateCreatesPendingCopy(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	source := newTestUpdate()
	launchAsset := &models.Assets{
		Id: "asset-launch", AppId: "app-1",
		Sha256B64url: "launchsha", ContentType: "application/javascript",
		StorageKey: "apps/my-app/assets/launchsha",
	}
	imageAsset := &models.Assets{
		Id: "asset-img", AppId: "app-1",
		Sha256B64url: "imgsha", ContentType: "image/png",
		StorageKey: "apps/my-app/assets/imgsha",
	}
	sourceAssets := []*models.UpdateAssets{
		{Id: "ua-1", UpdateId: "update-1", AssetId: "asset-launch", AssetKey: "bundlekey", SortOrder: 0},
		{Id: "ua-2", UpdateId: "update-1", AssetId: "asset-img", AssetKey: "imgkey", SortOrder: 1},
	}

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.Updates.EXPECT().FindOne(gomock.Any(), "update-1").Return(source, nil)
	m.RuntimeVersions.EXPECT().FindOne(gomock.Any(), "rv-1").
		Return(&models.RuntimeVersions{Id: "rv-1", AppId: "app-1", Version: "1.0.0"}, nil)
	m.UpdateAssets.EXPECT().FindAllByUpdateId(gomock.Any(), "update-1").Return(sourceAssets, nil)
	m.Assets.EXPECT().FindAllByIds(gomock.Any(), []string{"asset-launch", "asset-img"}).
		Return([]*models.Assets{launchAsset, imageAsset}, nil)

	var created *models.Updates
	m.Updates.EXPECT().InsertWithAssets(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, update *models.Updates, assetRows []*models.UpdateAssets) error {
			created = update
			if update.Status != "pending" || update.PublishedAt.Valid {
				t.Errorf("copy must be a pending draft: %+v", update)
			}
			if update.RolledBackFrom.String != "update-1" {
				t.Errorf("RolledBackFrom = %q", update.RolledBackFrom.String)
			}
			if update.ManifestUuid == source.ManifestUuid {
				t.Error("manifest uuid must be recomputed")
			}
			if len(assetRows) != 2 || assetRows[0].AssetId != "asset-launch" {
				t.Errorf("assetRows = %+v", assetRows)
			}
			var manifest map[string]any
			if err := json.Unmarshal([]byte(update.ManifestSnapshot), &manifest); err != nil {
				t.Fatalf("snapshot is not JSON: %v", err)
			}
			if manifest["id"] != update.ManifestUuid {
				t.Error("snapshot id != manifest uuid")
			}
			return nil
		})
	m.Updates.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id string) (*models.Updates, error) {
			return created, nil
		})

	resp, err := NewRollbackUpdateLogic(ctxWithUserID("user-1"), svcCtx).RollbackUpdate(&types.UpdateIdPath{
		AppSlug: "my-app", UpdateId: "update-1",
	})
	if err != nil {
		t.Fatalf("RollbackUpdate returned error: %v", err)
	}
	if resp.UpdateId == "" || resp.UpdateId == "update-1" {
		t.Errorf("UpdateId = %q", resp.UpdateId)
	}
}
