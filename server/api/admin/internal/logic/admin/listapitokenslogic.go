// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListApiTokensLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListApiTokensLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListApiTokensLogic {
	return &ListApiTokensLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListApiTokensLogic) ListApiTokens(req *types.AppSlugPath) (resp *types.ListTokensResp, err error) {
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	tokens, err := l.svcCtx.ApiTokensModel.FindAllByAppId(l.ctx, app.Id)
	if err != nil {
		return nil, err
	}

	items := make([]types.TokenItem, 0, len(tokens))
	for _, token := range tokens {
		items = append(items, types.TokenItem{
			Id:         token.Id,
			Name:       token.Name,
			CreatedBy:  token.CreatedBy,
			Scopes:     token.Scopes,
			LastUsedAt: formatNullTime(token.LastUsedAt),
			ExpiresAt:  formatNullTime(token.ExpiresAt),
			CreatedAt:  formatTime(token.CreatedAt),
			RevokedAt:  formatNullTime(token.RevokedAt),
		})
	}

	return &types.ListTokensResp{Items: items}, nil
}
