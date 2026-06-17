package admin

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
	"go.uber.org/mock/gomock"
)

func shaB64url(content string) (string, []byte) {
	sum := sha256.Sum256([]byte(content))
	return base64.RawURLEncoding.EncodeToString(sum[:]), sum[:]
}

func TestPlanUploadSplitsMissingAndReuse(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	bundleSha, bundleRaw := shaB64url("bundle")
	imgSha, imgRaw := shaB64url("img")

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.Assets.EXPECT().FindOneByAppIdSha256(gomock.Any(), "app-1", models.ByteaHex(bundleRaw)).
		Return(nil, models.ErrNotFound)
	m.Assets.EXPECT().FindOneByAppIdSha256(gomock.Any(), "app-1", models.ByteaHex(imgRaw)).
		Return(&models.Assets{Id: "asset-img"}, nil)

	resp, err := NewPlanUploadLogic(ctxWithUserID("user-1"), svcCtx).PlanUpload(&types.PlanReq{
		AppSlug:        "my-app",
		RuntimeVersion: "1.0.0",
		Platform:       "ios",
		Assets: []types.PlanAssetItem{
			{Key: "bundlekey", Sha256: bundleSha, Size: 100, ContentType: "application/javascript"},
			{Key: "imgkey", Sha256: imgSha, Size: 50, ContentType: "image/png"},
		},
	})
	if err != nil {
		t.Fatalf("PlanUpload returned error: %v", err)
	}

	if len(resp.Missing) != 1 || resp.Missing[0].Key != "bundlekey" {
		t.Errorf("Missing = %+v", resp.Missing)
	}
	wantFinal := "https://cos.example.com/apps/my-app/assets/" + bundleSha
	if resp.Missing[0].FinalUrl != wantFinal {
		t.Errorf("FinalUrl = %q, want %q", resp.Missing[0].FinalUrl, wantFinal)
	}
	if resp.Missing[0].PutUrl == "" || resp.Missing[0].PutHeaders["Content-Type"] != "application/javascript" {
		t.Errorf("Missing[0] = %+v", resp.Missing[0])
	}
	if len(resp.Reuse) != 1 || resp.Reuse[0].Key != "imgkey" {
		t.Errorf("Reuse = %+v", resp.Reuse)
	}
}

func TestPlanUploadRejectsBadSha256(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)

	_, err := NewPlanUploadLogic(ctxWithUserID("user-1"), svcCtx).PlanUpload(&types.PlanReq{
		AppSlug:        "my-app",
		RuntimeVersion: "1.0.0",
		Platform:       "ios",
		Assets:         []types.PlanAssetItem{{Key: "k", Sha256: "not-base64url!!", Size: 1, ContentType: "image/png"}},
	})
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Errorf("err = %v, want sha256 validation error", err)
	}
}

func TestFinalizeUploadSizeMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	bundleSha, _ := shaB64url("bundle")
	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	svcCtx.Store.(*fakeStore).headSizes["apps/my-app/assets/"+bundleSha] = 999 // declared size is 100

	_, err := NewFinalizeUploadLogic(ctxWithUserID("user-1"), svcCtx).FinalizeUpload(&types.FinalizeReq{
		AppSlug:        "my-app",
		RuntimeVersion: "1.0.0",
		Platform:       "ios",
		Assets: []types.PlanAssetItem{
			{Key: "bundlekey", Sha256: bundleSha, Size: 100, ContentType: "application/javascript"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "bundlekey") {
		t.Errorf("err = %v, want size mismatch listing bundlekey", err)
	}
	_ = m
}

func TestFinalizeUploadCreatesPendingDraft(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	bundleSha, bundleRaw := shaB64url("bundle")
	imgSha, imgRaw := shaB64url("img")
	store := svcCtx.Store.(*fakeStore)
	store.headSizes["apps/my-app/assets/"+bundleSha] = 100
	store.headSizes["apps/my-app/assets/"+imgSha] = 50

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)

	m.Assets.EXPECT().InsertIgnoreConflict(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	m.Assets.EXPECT().FindOneByAppIdSha256(gomock.Any(), "app-1", models.ByteaHex(bundleRaw)).
		Return(&models.Assets{
			Id: "asset-bundle", AppId: "app-1", Sha256B64url: bundleSha,
			SizeBytes: 100, ContentType: "application/javascript",
			StorageKey: "apps/my-app/assets/" + bundleSha,
		}, nil)
	m.Assets.EXPECT().FindOneByAppIdSha256(gomock.Any(), "app-1", models.ByteaHex(imgRaw)).
		Return(&models.Assets{
			Id: "asset-img", AppId: "app-1", Sha256B64url: imgSha,
			SizeBytes: 50, ContentType: "image/png",
			StorageKey: "apps/my-app/assets/" + imgSha,
		}, nil)

	m.RuntimeVersions.EXPECT().FindOneByAppIdVersion(gomock.Any(), "app-1", "1.0.0").
		Return(nil, models.ErrNotFound)
	m.RuntimeVersions.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil, nil)
	m.RuntimeVersions.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id string) (*models.RuntimeVersions, error) {
			return &models.RuntimeVersions{Id: id, AppId: "app-1", Version: "1.0.0"}, nil
		})
	m.Updates.EXPECT().FindOneByAppIdManifestUuid(gomock.Any(), "app-1", gomock.Any()).
		Return(nil, models.ErrNotFound)

	var created *models.Updates
	m.Updates.EXPECT().InsertWithAssets(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, update *models.Updates, assetRows []*models.UpdateAssets) error {
			created = update
			if update.Status != "pending" {
				t.Errorf("Status = %q, want pending", update.Status)
			}
			if update.LaunchAssetId != "asset-bundle" {
				t.Errorf("LaunchAssetId = %q", update.LaunchAssetId)
			}
			if len(assetRows) != 2 {
				t.Errorf("assetRows = %+v", assetRows)
			}

			var manifest map[string]any
			if err := json.Unmarshal([]byte(update.ManifestSnapshot), &manifest); err != nil {
				t.Fatalf("snapshot is not JSON: %v", err)
			}
			if manifest["id"] != update.ManifestUuid {
				t.Error("snapshot id != manifest uuid")
			}
			launch := manifest["launchAsset"].(map[string]any)
			if launch["url"] != "https://cos.example.com/apps/my-app/assets/"+bundleSha {
				t.Errorf("launchAsset url = %v", launch["url"])
			}
			if assets := manifest["assets"].([]any); len(assets) != 1 {
				t.Errorf("manifest assets = %v, want 1 entry (launch asset excluded)", assets)
			}
			return nil
		})
	m.Updates.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id string) (*models.Updates, error) {
			return created, nil
		})

	resp, err := NewFinalizeUploadLogic(ctxWithUserID("user-1"), svcCtx).FinalizeUpload(&types.FinalizeReq{
		AppSlug:        "my-app",
		RuntimeVersion: "1.0.0",
		Platform:       "ios",
		Message:        "fix checkout",
		Assets: []types.PlanAssetItem{
			{Key: "bundlekey", Sha256: bundleSha, Size: 100, ContentType: "application/javascript", FileExt: ".hbc"},
			{Key: "imgkey", Sha256: imgSha, Size: 50, ContentType: "image/png", FileExt: ".png"},
		},
	})
	if err != nil {
		t.Fatalf("FinalizeUpload returned error: %v", err)
	}
	if resp.Status != "pending" || resp.UpdateId == "" || resp.ManifestUuid == "" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestFinalizeUploadRejectsDuplicateManifestUuid(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	bundleSha, bundleRaw := shaB64url("bundle")
	store := svcCtx.Store.(*fakeStore)
	store.headSizes["apps/my-app/assets/"+bundleSha] = 100

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.Assets.EXPECT().InsertIgnoreConflict(gomock.Any(), gomock.Any()).Return(nil)
	m.Assets.EXPECT().FindOneByAppIdSha256(gomock.Any(), "app-1", models.ByteaHex(bundleRaw)).
		Return(&models.Assets{
			Id: "asset-bundle", AppId: "app-1", Sha256B64url: bundleSha,
			SizeBytes: 100, ContentType: "application/javascript",
			StorageKey: "apps/my-app/assets/" + bundleSha,
		}, nil)
	m.RuntimeVersions.EXPECT().FindOneByAppIdVersion(gomock.Any(), "app-1", "1.0.0").
		Return(&models.RuntimeVersions{Id: "rv-1", AppId: "app-1", Version: "1.0.0"}, nil)
	m.Updates.EXPECT().FindOneByAppIdManifestUuid(gomock.Any(), "app-1", gomock.Any()).
		Return(&models.Updates{Id: "existing-update", AppId: "app-1", ManifestUuid: "manifest-1"}, nil)

	_, err := NewFinalizeUploadLogic(ctxWithUserID("user-1"), svcCtx).FinalizeUpload(&types.FinalizeReq{
		AppSlug:        "my-app",
		RuntimeVersion: "1.0.0",
		Platform:       "ios",
		Assets: []types.PlanAssetItem{
			{Key: "bundlekey", Sha256: bundleSha, Size: 100, ContentType: "application/javascript", FileExt: ".hbc"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "existing-update") {
		t.Fatalf("err = %v, want duplicate finalize conflict with existing id", err)
	}
}

func TestFinalizeUploadRequiresLaunchAsset(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	imgSha, _ := shaB64url("img")
	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)

	_, err := NewFinalizeUploadLogic(ctxWithUserID("user-1"), svcCtx).FinalizeUpload(&types.FinalizeReq{
		AppSlug:        "my-app",
		RuntimeVersion: "1.0.0",
		Platform:       "ios",
		Assets: []types.PlanAssetItem{
			{Key: "imgkey", Sha256: imgSha, Size: 50, ContentType: "image/png"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "launch asset") {
		t.Errorf("err = %v, want launch asset error", err)
	}
	_ = sql.ErrNoRows
}
