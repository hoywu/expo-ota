// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"database/sql"
	"errors"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

type PatchSigningKeyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPatchSigningKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PatchSigningKeyLogic {
	return &PatchSigningKeyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PatchSigningKeyLogic) PatchSigningKey(req *types.PatchSigningKeyReq) (resp *types.SigningKeyResp, err error) {
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	key, err := l.svcCtx.CodeSigningKeysModel.FindOneByAppId(l.ctx, app.Id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errSigningKeyNotFound
		}
		return nil, err
	}

	if key.Enabled != req.Enabled {
		key.Enabled = req.Enabled
		if req.Enabled {
			key.DisabledAt = sql.NullTime{}
		} else {
			key.DisabledAt = nullableNow()
		}
		if err := l.svcCtx.CodeSigningKeysModel.Update(l.ctx, key); err != nil {
			return nil, err
		}
	}

	writeAudit(l.ctx, l.svcCtx, "patch_signing_key", app.Id, "code_signing_key", key.Id, map[string]any{
		"enabled": req.Enabled,
	})

	return signingKeyToResp(key), nil
}
