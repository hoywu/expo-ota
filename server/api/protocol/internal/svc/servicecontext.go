// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"encoding/base64"
	"fmt"

	"github.com/hoywu/expo-ota/server/api/protocol/internal/config"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config                config.Config
	DB                    sqlx.SqlConn
	AppsModel             models.AppsModel
	RuntimeVersionsModel  models.RuntimeVersionsModel
	UpdatesModel          models.UpdatesModel
	CodeSigningKeysModel  models.CodeSigningKeysModel
	ManifestRequestsModel models.ManifestRequestsModel
	ClientEventsModel     models.ClientEventsModel
	// SigningEncryptionKey is the decoded AES-256-GCM key used to decrypt
	// code signing private keys at rest (§5.5).
	SigningEncryptionKey []byte
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewSqlConn("postgres", c.DataSource)

	signingKey, err := base64.StdEncoding.DecodeString(c.SigningKeyEncryptionKey)
	logx.Must(err)
	if len(signingKey) != 32 {
		logx.Must(fmt.Errorf("SigningKeyEncryptionKey must be base64 of 32 bytes, got %d bytes", len(signingKey)))
	}

	return &ServiceContext{
		Config:                c,
		DB:                    db,
		AppsModel:             models.NewAppsModel(db),
		RuntimeVersionsModel:  models.NewRuntimeVersionsModel(db),
		UpdatesModel:          models.NewUpdatesModel(db),
		CodeSigningKeysModel:  models.NewCodeSigningKeysModel(db),
		ManifestRequestsModel: models.NewManifestRequestsModel(db),
		ClientEventsModel:     models.NewClientEventsModel(db),
		SigningEncryptionKey:  signingKey,
	}
}
