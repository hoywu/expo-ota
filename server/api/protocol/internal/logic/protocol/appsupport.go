package protocol

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/hoywu/expo-ota/server/api/protocol/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/protocol/internal/svc"
	"github.com/hoywu/expo-ota/server/db/models"
)

var errAppNotFound = httperr.New(http.StatusNotFound, "app not found")

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

// newUUID returns a uuidv7 string, matching the DB default key scheme.
func newUUID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
