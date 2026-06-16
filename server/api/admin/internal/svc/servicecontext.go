// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"encoding/base64"
	"fmt"

	"github.com/hoywu/expo-ota/server/api/admin/internal/config"
	"github.com/hoywu/expo-ota/server/db/models"
	"github.com/hoywu/expo-ota/server/internal/storage"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config                config.Config
	DB                    sqlx.SqlConn
	UsersModel            models.UsersModel
	AppsModel             models.AppsModel
	RuntimeVersionsModel  models.RuntimeVersionsModel
	UpdatesModel          models.UpdatesModel
	AssetsModel           models.AssetsModel
	UpdateAssetsModel     models.UpdateAssetsModel
	ApiTokensModel        models.ApiTokensModel
	CodeSigningKeysModel  models.CodeSigningKeysModel
	ManifestRequestsModel models.ManifestRequestsModel
	ClientEventsModel     models.ClientEventsModel
	AuditLogsModel        models.AuditLogsModel
	Store                 storage.Store
	// SigningEncryptionKey is the decoded AES-256-GCM key used to encrypt
	// code signing private keys at rest.
	SigningEncryptionKey []byte
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewSqlConn("postgres", c.DataSource)

	signingKey, err := base64.StdEncoding.DecodeString(c.SigningKeyEncryptionKey)
	logx.Must(err)
	if len(signingKey) != 32 {
		logx.Must(fmt.Errorf("SigningKeyEncryptionKey must be base64 of 32 bytes, got %d bytes", len(signingKey)))
	}

	storage.SetAssetKeyPrefix(c.Cos.KeyPrefix)

	store, err := storage.NewCosStore(storage.CosConfig{
		SecretID:  c.Cos.SecretID,
		SecretKey: c.Cos.SecretKey,
		Region:    c.Cos.Region,
		Bucket:    c.Cos.Bucket,
		Domain:    c.Cos.Domain,
	})
	logx.Must(err)

	return &ServiceContext{
		Config:                c,
		DB:                    db,
		UsersModel:            models.NewUsersModel(db),
		AppsModel:             models.NewAppsModel(db),
		RuntimeVersionsModel:  models.NewRuntimeVersionsModel(db),
		UpdatesModel:          models.NewUpdatesModel(db),
		AssetsModel:           models.NewAssetsModel(db),
		UpdateAssetsModel:     models.NewUpdateAssetsModel(db),
		ApiTokensModel:        models.NewApiTokensModel(db),
		CodeSigningKeysModel:  models.NewCodeSigningKeysModel(db),
		ManifestRequestsModel: models.NewManifestRequestsModel(db),
		ClientEventsModel:     models.NewClientEventsModel(db),
		AuditLogsModel:        models.NewAuditLogsModel(db),
		Store:                 store,
		SigningEncryptionKey:  signingKey,
	}
}
