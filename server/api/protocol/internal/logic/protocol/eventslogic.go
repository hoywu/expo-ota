// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package protocol

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/hoywu/expo-ota/server/api/protocol/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/protocol/internal/svc"
	"github.com/hoywu/expo-ota/server/api/protocol/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

var (
	errMissingEventId  = httperr.New(http.StatusBadRequest, "eventId is required")
	errMissingDeviceId = httperr.New(http.StatusBadRequest, "deviceId is required")
	errInvalidOccurred = httperr.New(http.StatusBadRequest, "occurredAt must be an RFC3339 timestamp")
)

type EventsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EventsLogic {
	return &EventsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Events ingests one client update event (§5.6). It is idempotent on
// (app_id, event_id): retried events are silently deduplicated.
func (l *EventsLogic) Events(req *types.EventReq) (resp *types.EmptyResp, err error) {
	if req.EventId == "" {
		return nil, errMissingEventId
	}
	if req.DeviceId == "" {
		return nil, errMissingDeviceId
	}

	occurredAt, err := time.Parse(time.RFC3339, req.OccurredAt)
	if err != nil {
		return nil, errInvalidOccurred
	}

	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	manifestUuid := req.ManifestUuid
	updateId := req.UpdateId
	// Legacy clients sent expo manifest.id in updateId before reload.
	if manifestUuid == "" && updateId != "" {
		manifestUuid = updateId
		updateId = ""
	}
	if manifestUuid != "" {
		update, ferr := l.svcCtx.UpdatesModel.FindOneByAppIdManifestUuid(l.ctx, app.Id, manifestUuid)
		if ferr == nil {
			updateId = update.Id
		} else if !errors.Is(ferr, models.ErrNotFound) {
			l.Errorf("resolve manifest uuid %s to update failed: %v", manifestUuid, ferr)
		}
	}

	row := &models.ClientEvents{
		AppId:          app.Id,
		OccurredAt:     occurredAt,
		ReceivedAt:     time.Now(),
		EventId:        req.EventId,
		EventType:      req.EventType,
		UpdateId:       nullString(updateId),
		ManifestUuid:   nullString(manifestUuid),
		RuntimeVersion: nullString(req.RuntimeVersion),
		Platform:       nullString(req.Platform),
		DeviceId:       req.DeviceId,
		AppVersion:     nullString(req.AppVersion),
		OsVersion:      nullString(req.OsVersion),
		DurationMs:     nullInt64(req.DurationMs),
		ErrorCode:      nullString(req.ErrorCode),
		ErrorMessage:   nullString(req.ErrorMessage),
	}

	inserted, err := l.svcCtx.ClientEventsModel.InsertIgnoreConflict(l.ctx, row)
	if err != nil {
		return nil, err
	}
	if !inserted {
		l.Infof("duplicate client event %s for app %s ignored", req.EventId, app.AppSlug)
	}

	return &types.EmptyResp{}, nil
}

func nullInt64(v int64) sql.NullInt64 {
	if v <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: v, Valid: true}
}
