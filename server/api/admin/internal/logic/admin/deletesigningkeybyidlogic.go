package admin

import (
	"context"
	"errors"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteSigningKeyByIDLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteSigningKeyByIDLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteSigningKeyByIDLogic {
	return &DeleteSigningKeyByIDLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteSigningKeyByIDLogic) DeleteSigningKeyByID(req *types.SigningKeyIdPath) (resp *types.EmptyResp, err error) {
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	key, err := l.svcCtx.CodeSigningKeysModel.FindOneByAppIdKeyId(l.ctx, app.Id, req.KeyId)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errSigningKeyNotFound
		}
		return nil, err
	}

	if key.Enabled || !key.DisabledAt.Valid {
		return nil, errSigningKeyNotCooledDown
	}

	if err := l.svcCtx.CodeSigningKeysModel.Delete(l.ctx, key.Id); err != nil {
		return nil, err
	}

	writeAudit(l.ctx, l.svcCtx, "delete_signing_key", app.Id, "code_signing_key", key.Id, map[string]any{
		"keyId": key.KeyId,
	})

	return &types.EmptyResp{}, nil
}

