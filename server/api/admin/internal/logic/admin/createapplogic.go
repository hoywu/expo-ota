// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"errors"
	"net/http"
	"regexp"

	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

// appSlugPattern mirrors the DB CHECK constraint on apps.app_slug.
var appSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$`)

var (
	errInvalidAppSlug = httperr.New(http.StatusBadRequest, "appSlug must match ^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$")
	errAppSlugTaken   = httperr.New(http.StatusConflict, "appSlug already exists")
	errAppNameEmpty   = httperr.New(http.StatusBadRequest, "name must not be empty")
)

type CreateAppLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateAppLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAppLogic {
	return &CreateAppLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateAppLogic) CreateApp(req *types.CreateAppReq) (resp *types.AppResp, err error) {
	if !appSlugPattern.MatchString(req.AppSlug) {
		return nil, errInvalidAppSlug
	}
	if req.Name == "" {
		return nil, errAppNameEmpty
	}

	if _, err := l.svcCtx.AppsModel.FindOneByAppSlug(l.ctx, req.AppSlug); err == nil {
		return nil, errAppSlugTaken
	} else if !errors.Is(err, models.ErrNotFound) {
		return nil, err
	}

	id, err := newUUID()
	if err != nil {
		return nil, err
	}

	app := &models.Apps{
		Id:          id,
		AppSlug:     req.AppSlug,
		Name:        req.Name,
		Description: nullString(req.Description),
	}
	if _, err := l.svcCtx.AppsModel.Insert(l.ctx, app); err != nil {
		return nil, err
	}

	created, err := l.svcCtx.AppsModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	writeAudit(l.ctx, l.svcCtx, "create_app", created.Id, "app", created.Id, map[string]any{
		"appSlug": created.AppSlug,
		"name":    created.Name,
	})

	return appToResp(created), nil
}
