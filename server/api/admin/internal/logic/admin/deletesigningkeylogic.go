// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

// signingKeyDeleteCooldown protects online clients: a key must be disabled
// for at least this long before it can be hard-deleted (§5.5).
const signingKeyDeleteCooldown = 24 * time.Hour

var errSigningKeyNotCooledDown = httperr.New(http.StatusBadRequest,
	"signing key must be disabled for at least 24 hours before deletion")

type DeleteSigningKeyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteSigningKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteSigningKeyLogic {
	return &DeleteSigningKeyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteSigningKeyLogic) DeleteSigningKey(req *types.AppSlugPath) (resp *types.EmptyResp, err error) {
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

	if key.Enabled || !key.DisabledAt.Valid || time.Since(key.DisabledAt.Time) < signingKeyDeleteCooldown {
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
