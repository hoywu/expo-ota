// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

var (
	errInvalidTimeRange   = httperr.New(http.StatusBadRequest, "from/to must be valid RFC 3339 timestamps")
	errInvalidAuditCursor = httperr.New(http.StatusBadRequest, "cursor is invalid")
)

type ListAuditLogsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListAuditLogsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAuditLogsLogic {
	return &ListAuditLogsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListAuditLogsLogic) ListAuditLogs(req *types.ListAuditLogsReq) (resp *types.ListAuditLogsResp, err error) {
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	filter := models.AuditLogsFilter{
		Action:      req.Action,
		ActorUserId: req.Actor,
		Limit:       req.Limit,
	}
	if req.From != "" {
		if filter.From, err = time.Parse(time.RFC3339, req.From); err != nil {
			return nil, errInvalidTimeRange
		}
	}
	if req.To != "" {
		if filter.To, err = time.Parse(time.RFC3339, req.To); err != nil {
			return nil, errInvalidTimeRange
		}
	}
	if req.Cursor != "" {
		if filter.CursorId, err = strconv.ParseInt(req.Cursor, 10, 64); err != nil {
			return nil, errInvalidAuditCursor
		}
	}

	logs, err := l.svcCtx.AuditLogsModel.FindManyByAppId(l.ctx, app.Id, filter)
	if err != nil {
		return nil, err
	}

	items := make([]types.AuditLogItem, 0, len(logs))
	for _, row := range logs {
		item := types.AuditLogItem{
			Id:          strconv.FormatInt(row.Id, 10),
			AppSlug:     app.AppSlug,
			ActorUserId: row.ActorUserId.String,
			Action:      row.Action,
			TargetType:  row.TargetType.String,
			TargetId:    row.TargetId.String,
			RequestId:   row.RequestId.String,
			Ip:          row.Ip.String,
			UserAgent:   row.UserAgent.String,
			OccurredAt:  formatTime(row.OccurredAt),
		}
		if row.Payload != "" {
			_ = json.Unmarshal([]byte(row.Payload), &item.Payload)
		}
		items = append(items, item)
	}

	nextCursor := ""
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(logs) == limit {
		nextCursor = strconv.FormatInt(logs[len(logs)-1].Id, 10)
	}

	return &types.ListAuditLogsResp{Items: items, NextCursor: nextCursor}, nil
}
