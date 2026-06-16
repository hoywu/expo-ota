// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"errors"
	"net/http"

	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

var errTokenNotFound = httperr.New(http.StatusNotFound, "api token not found")

type RevokeApiTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRevokeApiTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RevokeApiTokenLogic {
	return &RevokeApiTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RevokeApiTokenLogic) RevokeApiToken(req *types.TokenIdPath) (resp *types.EmptyResp, err error) {
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	token, err := l.svcCtx.ApiTokensModel.FindOne(l.ctx, req.TokenId)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errTokenNotFound
		}
		return nil, err
	}
	if token.AppId != app.Id {
		return nil, errTokenNotFound
	}

	if !token.RevokedAt.Valid {
		token.RevokedAt = nullableNow()
		if err := l.svcCtx.ApiTokensModel.Update(l.ctx, token); err != nil {
			return nil, err
		}
	}

	writeAudit(l.ctx, l.svcCtx, "revoke_api_token", app.Id, "api_token", token.Id, nil)

	return &types.EmptyResp{}, nil
}
