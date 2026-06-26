package admin

import (
	"context"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
)

func emptyUpdateStats() *types.UpdateStatsResp {
	return &types.UpdateStatsResp{}
}

func buildUpdateStats(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	app *models.Apps,
	update *models.Updates,
) (*types.UpdateStatsResp, error) {
	requestedDevices, err := svcCtx.ManifestRequestsModel.CountDistinctDevices(ctx, app.Id, update.Id)
	if err != nil {
		return nil, err
	}
	requestsWithoutDeviceId, err := svcCtx.ManifestRequestsModel.CountWithoutDeviceId(ctx, app.Id, update.Id)
	if err != nil {
		return nil, err
	}
	eventStats, err := svcCtx.ClientEventsModel.StatsByUpdate(ctx, app.Id, update.ManifestUuid)
	if err != nil {
		return nil, err
	}

	return &types.UpdateStatsResp{
		RequestedDevices:        int(requestedDevices),
		RequestsWithoutDeviceId: int(requestsWithoutDeviceId),
		SucceededDevices:        int(eventStats.SucceededDevices),
		FailedDevices:           int(eventStats.FailedDevices),
		DurationMinMs:           int(eventStats.DurationMinMs.Int64),
		DurationMaxMs:           int(eventStats.DurationMaxMs.Int64),
		DurationAvgMs:           int(eventStats.DurationAvgMs.Int64),
	}, nil
}
