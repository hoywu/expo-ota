// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"net/http"

	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReadyzLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReadyzLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReadyzLogic {
	return &ReadyzLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReadyzLogic) Readyz() (resp *types.ReadyResp, err error) {
	var one int
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &one, "select 1"); err != nil {
		l.Errorf("readyz db ping failed: %v", err)
		return nil, httperr.New(http.StatusServiceUnavailable, "database unavailable")
	}

	return &types.ReadyResp{Status: "ok", Db: "ok"}, nil
}
