package admin

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	"github.com/hoywu/expo-ota/server/api/admin/internal/config"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/db/models"
	"go.uber.org/mock/gomock"
)

// testMocks bundles every model mock wired into the test ServiceContext.
type testMocks struct {
	Users            *models.MockUsersModel
	Apps             *models.MockAppsModel
	RuntimeVersions  *models.MockRuntimeVersionsModel
	Updates          *models.MockUpdatesModel
	Assets           *models.MockAssetsModel
	UpdateAssets     *models.MockUpdateAssetsModel
	ApiTokens        *models.MockApiTokensModel
	CodeSigningKeys  *models.MockCodeSigningKeysModel
	ManifestRequests *models.MockManifestRequestsModel
	ClientEvents     *models.MockClientEventsModel
	AuditLogs        *models.MockAuditLogsModel
	Store            *fakeStore
}

// newFullTestSvcCtx builds a ServiceContext with all dependencies mocked.
// Audit writes are accepted by default since they are best-effort.
func newFullTestSvcCtx(ctrl *gomock.Controller) (*svc.ServiceContext, *testMocks) {
	m := &testMocks{
		Users:            models.NewMockUsersModel(ctrl),
		Apps:             models.NewMockAppsModel(ctrl),
		RuntimeVersions:  models.NewMockRuntimeVersionsModel(ctrl),
		Updates:          models.NewMockUpdatesModel(ctrl),
		Assets:           models.NewMockAssetsModel(ctrl),
		UpdateAssets:     models.NewMockUpdateAssetsModel(ctrl),
		ApiTokens:        models.NewMockApiTokensModel(ctrl),
		CodeSigningKeys:  models.NewMockCodeSigningKeysModel(ctrl),
		ManifestRequests: models.NewMockManifestRequestsModel(ctrl),
		ClientEvents:     models.NewMockClientEventsModel(ctrl),
		AuditLogs:        models.NewMockAuditLogsModel(ctrl),
		Store:            &fakeStore{headSizes: map[string]int64{}},
	}
	m.AuditLogs.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	c := config.Config{
		RefreshSecret:           testRefreshSecret,
		RefreshExpire:           testRefreshExpire,
		SigningKeyEncryptionKey: base64.StdEncoding.EncodeToString(make([]byte, 32)),
		PresignExpireSeconds:    900,
	}
	c.Auth.AccessSecret = testAccessSecret
	c.Auth.AccessExpire = testAccessExpire

	return &svc.ServiceContext{
		Config:                c,
		UsersModel:            m.Users,
		AppsModel:             m.Apps,
		RuntimeVersionsModel:  m.RuntimeVersions,
		UpdatesModel:          m.Updates,
		AssetsModel:           m.Assets,
		UpdateAssetsModel:     m.UpdateAssets,
		ApiTokensModel:        m.ApiTokens,
		CodeSigningKeysModel:  m.CodeSigningKeys,
		ManifestRequestsModel: m.ManifestRequests,
		ClientEventsModel:     m.ClientEvents,
		AuditLogsModel:        m.AuditLogs,
		Store:                 m.Store,
		SigningEncryptionKey:  make([]byte, 32),
	}, m
}

// fakeStore is an in-memory storage.Store for tests.
type fakeStore struct {
	headSizes map[string]int64 // storageKey -> size; absent means missing
}

func (f *fakeStore) PublicURL(storageKey string) string {
	return "https://cos.example.com/" + storageKey
}

func (f *fakeStore) PresignPut(_ context.Context, storageKey, contentType string, _ time.Duration) (string, map[string]string, error) {
	return "https://cos.example.com/put/" + storageKey, map[string]string{"Content-Type": contentType}, nil
}

func (f *fakeStore) Head(_ context.Context, storageKey string) (int64, error) {
	size, ok := f.headSizes[storageKey]
	if !ok {
		return 0, errors.New("object not found")
	}
	return size, nil
}

func newTestApp() *models.Apps {
	return &models.Apps{
		Id:        "app-1",
		AppSlug:   "my-app",
		Name:      "My App",
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}
