package admin

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
	"go.uber.org/mock/gomock"
)

func TestCreateAppSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(nil, models.ErrNotFound)
	var insertedId string
	m.Apps.EXPECT().Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, app *models.Apps) (sql.Result, error) {
			insertedId = app.Id
			if app.AppSlug != "my-app" || app.Name != "My App" {
				t.Errorf("unexpected insert: %+v", app)
			}
			return nil, nil
		})
	m.Apps.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id string) (*models.Apps, error) {
			if id != insertedId {
				t.Errorf("FindOne id = %q, want %q", id, insertedId)
			}
			app := newTestApp()
			app.Id = id
			return app, nil
		})

	resp, err := NewCreateAppLogic(ctxWithUserID("user-1"), svcCtx).CreateApp(&types.CreateAppReq{
		AppSlug: "my-app",
		Name:    "My App",
	})
	if err != nil {
		t.Fatalf("CreateApp returned error: %v", err)
	}
	if resp.AppSlug != "my-app" {
		t.Errorf("AppSlug = %q", resp.AppSlug)
	}
}

func TestCreateAppInvalidSlug(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, _ := newFullTestSvcCtx(ctrl)

	for _, slug := range []string{"", "ab", "-bad-", "UPPER", "a_b_c"} {
		_, err := NewCreateAppLogic(ctxWithUserID("user-1"), svcCtx).CreateApp(&types.CreateAppReq{
			AppSlug: slug, Name: "x",
		})
		if !errors.Is(err, errInvalidAppSlug) {
			t.Errorf("slug %q: err = %v, want errInvalidAppSlug", slug, err)
		}
	}
}

func TestCreateAppSlugTaken(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)

	_, err := NewCreateAppLogic(ctxWithUserID("user-1"), svcCtx).CreateApp(&types.CreateAppReq{
		AppSlug: "my-app", Name: "My App",
	})
	if !errors.Is(err, errAppSlugTaken) {
		t.Errorf("err = %v, want errAppSlugTaken", err)
	}
}

func TestGetAppDeletedIsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	app := newTestApp()
	app.DeletedAt = sql.NullTime{Time: time.Now(), Valid: true}
	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(app, nil)

	_, err := NewGetAppLogic(ctxWithUserID("user-1"), svcCtx).GetApp(&types.AppSlugPath{AppSlug: "my-app"})
	if !errors.Is(err, errAppNotFound) {
		t.Errorf("err = %v, want errAppNotFound", err)
	}
}

func TestUpdateAppKeepsSlug(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.Apps.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, app *models.Apps) error {
			if app.AppSlug != "my-app" {
				t.Errorf("slug changed to %q", app.AppSlug)
			}
			if app.Name != "Renamed" {
				t.Errorf("Name = %q", app.Name)
			}
			return nil
		})

	resp, err := NewUpdateAppLogic(ctxWithUserID("user-1"), svcCtx).UpdateApp(&types.UpdateAppReq{
		AppSlug: "my-app", Name: "Renamed",
	})
	if err != nil {
		t.Fatalf("UpdateApp returned error: %v", err)
	}
	if resp.Name != "Renamed" {
		t.Errorf("Name = %q", resp.Name)
	}
}

func TestDeleteAppSoftDeletes(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.Apps.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, app *models.Apps) error {
			if !app.DeletedAt.Valid {
				t.Error("DeletedAt not set")
			}
			return nil
		})

	if _, err := NewDeleteAppLogic(ctxWithUserID("user-1"), svcCtx).DeleteApp(&types.AppSlugPath{AppSlug: "my-app"}); err != nil {
		t.Fatalf("DeleteApp returned error: %v", err)
	}
}

func TestListAppsMapsItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindAllActive(gomock.Any()).Return([]*models.Apps{newTestApp()}, nil)

	resp, err := NewListAppsLogic(ctxWithUserID("user-1"), svcCtx).ListApps()
	if err != nil {
		t.Fatalf("ListApps returned error: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].AppSlug != "my-app" {
		t.Errorf("Items = %+v", resp.Items)
	}
	if want := "2026-01-02T03:04:05Z"; resp.Items[0].CreatedAt != want {
		t.Errorf("CreatedAt = %q, want %q", resp.Items[0].CreatedAt, want)
	}
}
