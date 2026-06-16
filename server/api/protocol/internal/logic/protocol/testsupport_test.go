package protocol

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"time"

	"github.com/hoywu/expo-ota/server/api/protocol/internal/config"
	"github.com/hoywu/expo-ota/server/api/protocol/internal/svc"
	"github.com/hoywu/expo-ota/server/db/models"
	"go.uber.org/mock/gomock"
)

// testMocks bundles every model mock wired into the test ServiceContext.
type testMocks struct {
	Apps             *models.MockAppsModel
	RuntimeVersions  *models.MockRuntimeVersionsModel
	Updates          *models.MockUpdatesModel
	CodeSigningKeys  *models.MockCodeSigningKeysModel
	ManifestRequests *models.MockManifestRequestsModel
	ClientEvents     *models.MockClientEventsModel
}

// newTestSvcCtx builds a ServiceContext with all dependencies mocked.
// manifest_request observation writes are best-effort and accepted by default.
func newTestSvcCtx(ctrl *gomock.Controller) (*svc.ServiceContext, *testMocks) {
	m := &testMocks{
		Apps:             models.NewMockAppsModel(ctrl),
		RuntimeVersions:  models.NewMockRuntimeVersionsModel(ctrl),
		Updates:          models.NewMockUpdatesModel(ctrl),
		CodeSigningKeys:  models.NewMockCodeSigningKeysModel(ctrl),
		ManifestRequests: models.NewMockManifestRequestsModel(ctrl),
		ClientEvents:     models.NewMockClientEventsModel(ctrl),
	}
	m.ManifestRequests.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	return &svc.ServiceContext{
		Config:                config.Config{},
		AppsModel:             m.Apps,
		RuntimeVersionsModel:  m.RuntimeVersions,
		UpdatesModel:          m.Updates,
		CodeSigningKeysModel:  m.CodeSigningKeys,
		ManifestRequestsModel: m.ManifestRequests,
		ClientEventsModel:     m.ClientEvents,
		SigningEncryptionKey:  make([]byte, 32),
	}, m
}

func newTestApp() *models.Apps {
	return &models.Apps{
		Id:        "app-1",
		AppSlug:   "my-app",
		Name:      "My App",
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

func newTestRuntimeVersion() *models.RuntimeVersions {
	return &models.RuntimeVersions{Id: "rv-1", AppId: "app-1", Version: "1.0.0"}
}

// encryptForTest mirrors the admin side's AES-256-GCM encryption so tests can
// build a code_signing_keys row whose private key the protocol service can
// decrypt and use for signing.
func encryptForTest(key, plaintext []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil)
}
