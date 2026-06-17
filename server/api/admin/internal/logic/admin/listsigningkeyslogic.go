package admin

import (
	"context"
	"errors"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSigningKeysLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListSigningKeysLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSigningKeysLogic {
	return &ListSigningKeysLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListSigningKeysLogic) ListSigningKeys(req *types.AppSlugPath) (resp *types.ListSigningKeysResp, err error) {
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	keys, err := l.svcCtx.CodeSigningKeysModel.ListByAppId(l.ctx, app.Id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return &types.ListSigningKeysResp{Items: []types.SigningKeyResp{}}, nil
		}
		return nil, err
	}

	items := make([]types.SigningKeyResp, 0, len(keys))
	for _, key := range keys {
		items = append(items, *signingKeyToResp(key))
	}
	return &types.ListSigningKeysResp{Items: items}, nil
}

