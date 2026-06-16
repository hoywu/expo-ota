// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"errors"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSigningKeyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSigningKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSigningKeyLogic {
	return &GetSigningKeyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSigningKeyLogic) GetSigningKey(req *types.AppSlugPath) (resp *types.SigningKeyResp, err error) {
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

	return signingKeyToResp(key), nil
}
