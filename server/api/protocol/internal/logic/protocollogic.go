// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package logic

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/protocol/internal/svc"
	"github.com/hoywu/expo-ota/server/api/protocol/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProtocolLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewProtocolLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProtocolLogic {
	return &ProtocolLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProtocolLogic) Protocol(req *types.Request) (resp *types.Response, err error) {
	// todo: add your logic here and delete this line

	return
}
