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

type GenerateSigningKeyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGenerateSigningKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateSigningKeyLogic {
	return &GenerateSigningKeyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GenerateSigningKeyLogic) GenerateSigningKey(req *types.GenerateSigningKeyReq) (resp *types.SigningKeyResp, err error) {
	if req.KeyId == "" {
		return nil, errKeyIdEmpty
	}

	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	if err := ensureNoEnabledSigningKey(l.ctx, l.svcCtx, app.Id); err != nil {
		return nil, err
	}

	publicPem, privatePem, err := generateRsaKeyPair()
	if err != nil {
		return nil, err
	}

	encrypted, err := encryptPrivateKeyPem(l.svcCtx.SigningEncryptionKey, privatePem)
	if err != nil {
		return nil, err
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
		PublicKeyPem:        string(publicPem),
		EncryptedPrivateKey: models.ByteaHex(encrypted),
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

	writeAudit(l.ctx, l.svcCtx, "generate_signing_key", app.Id, "code_signing_key", id, map[string]any{
		"keyId": req.KeyId,
	})

	return signingKeyToResp(created), nil
}
