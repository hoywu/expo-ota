package protocol

import (
	"context"
	"errors"
	"testing"

	"github.com/hoywu/expo-ota/server/api/protocol/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
	"go.uber.org/mock/gomock"
)

func baseEventReq() *types.EventReq {
	return &types.EventReq{
		AppSlug:    "my-app",
		EventId:    "22222222-2222-2222-2222-222222222222",
		EventType:  "update_succeeded",
		OccurredAt: "2026-06-04T10:00:00.000Z",
		DeviceId:   "device-1",
		DurationMs: 4321,
	}
}

func TestEventsInsertsRow(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.ClientEvents.EXPECT().InsertIgnoreConflict(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, row *models.ClientEvents) (bool, error) {
			if row.AppId != "app-1" || row.EventId != baseEventReq().EventId {
				t.Errorf("unexpected row: %+v", row)
			}
			if !row.DurationMs.Valid || row.DurationMs.Int64 != 4321 {
				t.Errorf("durationMs = %+v", row.DurationMs)
			}
			if row.ReceivedAt.IsZero() {
				t.Error("receivedAt not set")
			}
			return true, nil
		})

	_, err := NewEventsLogic(context.Background(), svcCtx).Events(baseEventReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEventsIsIdempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.ClientEvents.EXPECT().InsertIgnoreConflict(gomock.Any(), gomock.Any()).Return(false, nil)

	_, err := NewEventsLogic(context.Background(), svcCtx).Events(baseEventReq())
	if err != nil {
		t.Errorf("duplicate event should not error: %v", err)
	}
}

func TestEventsRejectsBadTimestamp(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, _ := newTestSvcCtx(ctrl)

	req := baseEventReq()
	req.OccurredAt = "not-a-time"
	_, err := NewEventsLogic(context.Background(), svcCtx).Events(req)
	if !errors.Is(err, errInvalidOccurred) {
		t.Errorf("err = %v, want errInvalidOccurred", err)
	}
}

func TestEventsRejectsMissingDeviceId(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, _ := newTestSvcCtx(ctrl)

	req := baseEventReq()
	req.DeviceId = ""
	_, err := NewEventsLogic(context.Background(), svcCtx).Events(req)
	if !errors.Is(err, errMissingDeviceId) {
		t.Errorf("err = %v, want errMissingDeviceId", err)
	}
}

func TestEventsAppNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(nil, models.ErrNotFound)

	_, err := NewEventsLogic(context.Background(), svcCtx).Events(baseEventReq())
	if !errors.Is(err, errAppNotFound) {
		t.Errorf("err = %v, want errAppNotFound", err)
	}
}
