// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
	"github.com/hoywu/expo-ota/server/internal/storage"

	"github.com/zeromicro/go-zero/core/logx"
)

var errNoAssets = httperr.New(http.StatusBadRequest, "assets must not be empty")

type PlanUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlanUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlanUploadLogic {
	return &PlanUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PlanUploadLogic) PlanUpload(req *types.PlanReq) (resp *types.PlanResp, err error) {
	if len(req.Assets) == 0 {
		return nil, errNoAssets
	}

	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	presignExpire := time.Duration(l.svcCtx.Config.PresignExpireSeconds) * time.Second
	resp = &types.PlanResp{
		Missing: []types.PlanMissingItem{},
		Reuse:   []types.PlanReuseItem{},
	}

	for _, asset := range req.Assets {
		rawSha, err := decodeSha256B64url(asset.Sha256)
		if err != nil {
			return nil, err
		}

		storageKey := storage.AssetStorageKey(app.AppSlug, asset.Sha256)
		finalUrl := l.svcCtx.Store.PublicURL(storageKey)

		_, err = l.svcCtx.AssetsModel.FindOneByAppIdSha256(l.ctx, app.Id, rawSha)
		if err == nil {
			resp.Reuse = append(resp.Reuse, types.PlanReuseItem{
				Key:      asset.Key,
				Sha256:   asset.Sha256,
				FinalUrl: finalUrl,
			})
			continue
		}
		if !errors.Is(err, models.ErrNotFound) {
			return nil, err
		}

		contentMD5, err := md5HexToBase64(asset.Key)
		if err != nil {
			return nil, err
		}
		putUrl, putHeaders, err := l.svcCtx.Store.PresignPut(l.ctx, storageKey, asset.ContentType, contentMD5, presignExpire)
		if err != nil {
			return nil, err
		}
		resp.Missing = append(resp.Missing, types.PlanMissingItem{
			Key:         asset.Key,
			Sha256:      asset.Sha256,
			Size:        asset.Size,
			ContentType: asset.ContentType,
			PutUrl:      putUrl,
			PutHeaders:  putHeaders,
			FinalUrl:    finalUrl,
		})
	}

	return resp, nil
}
