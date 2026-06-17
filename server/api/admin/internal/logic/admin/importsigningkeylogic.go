// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

type ImportSigningKeyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewImportSigningKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ImportSigningKeyLogic {
	return &ImportSigningKeyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ImportSigningKeyLogic) ImportSigningKey(req *types.ImportSigningKeyReq) (resp *types.SigningKeyResp, err error) {
	if req.KeyId == "" {
		return nil, errKeyIdEmpty
	}
	if req.Algorithm != "" && req.Algorithm != signingKeyAlgorithm {
		return nil, errUnsupportedAlg
	}

	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	if err := ensureNoEnabledSigningKey(l.ctx, l.svcCtx, app.Id); err != nil {
		return nil, err
	}
	if err := recycleDisabledSigningKeyKeyID(l.ctx, l.svcCtx, app.Id, req.KeyId); err != nil {
		return nil, err
	}

	publicKey, err := parseRsaPublicKey([]byte(req.PublicKeyPem))
	if err != nil {
		return nil, err
	}

	// Private key is optional: without it the server cannot sign manifests
	// (HasPrivateKey=false), but the public key remains downloadable.
	encryptedPrivateKey := emptyBytea
	if req.PrivateKeyPem != "" {
		privateKey, err := parseRsaPrivateKey([]byte(req.PrivateKeyPem))
		if err != nil {
			return nil, err
		}
		if privateKey.PublicKey.N.Cmp(publicKey.N) != 0 {
			return nil, errInvalidPrivateKey
		}
		encrypted, err := encryptPrivateKeyPem(l.svcCtx.SigningEncryptionKey, []byte(req.PrivateKeyPem))
		if err != nil {
			return nil, err
		}
		encryptedPrivateKey = models.ByteaHex(encrypted)
	}

	id, err := newUUID()
	if err != nil {
		return nil, err
	}

	key := &models.CodeSigningKeys{
		Id:                  id,
		AppId:               app.Id,
		KeyId:               req.KeyId,
		Algorithm:           signingKeyAlgorithm,
		PublicKeyPem:        req.PublicKeyPem,
		EncryptedPrivateKey: encryptedPrivateKey,
		EncryptionKeyId:     signingEncryptionKeyID,
		Enabled:             true,
	}
	if _, err := l.svcCtx.CodeSigningKeysModel.Insert(l.ctx, key); err != nil {
		return nil, err
	}

	created, err := l.svcCtx.CodeSigningKeysModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	writeAudit(l.ctx, l.svcCtx, "import_signing_key", app.Id, "code_signing_key", id, map[string]any{
		"keyId": req.KeyId,
	})

	return signingKeyToResp(created), nil
}
