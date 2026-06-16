package admin

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/db/models"
)

var (
	errAppNotFound    = httperr.New(http.StatusNotFound, "app not found")
	errUpdateNotFound = httperr.New(http.StatusNotFound, "update not found")
)

// findActiveApp resolves an appSlug to a non-deleted app, mapping both
// "missing" and "soft-deleted" to a 404.
func findActiveApp(ctx context.Context, svcCtx *svc.ServiceContext, appSlug string) (*models.Apps, error) {
	app, err := svcCtx.AppsModel.FindOneByAppSlug(ctx, appSlug)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errAppNotFound
		}
		return nil, err
	}
	if app.DeletedAt.Valid {
		return nil, errAppNotFound
	}
	return app, nil
}

// findAppUpdate resolves an update that belongs to the given app and is not
// soft-deleted.
func findAppUpdate(ctx context.Context, svcCtx *svc.ServiceContext, app *models.Apps, updateId string) (*models.Updates, error) {
	update, err := svcCtx.UpdatesModel.FindOne(ctx, updateId)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, errUpdateNotFound
		}
		return nil, err
	}
	if update.AppId != app.Id || update.DeletedAt.Valid {
		return nil, errUpdateNotFound
	}
	return update, nil
}

// newUUID returns a uuidv7 string, matching the DB default key scheme.
func newUUID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func formatNullTime(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return formatTime(t.Time)
}
