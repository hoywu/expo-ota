package models

import (
	"context"
	"fmt"

	"github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AssetsModel = (*customAssetsModel)(nil)

type (
	// AssetsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAssetsModel.
	AssetsModel interface {
		assetsModel
		withSession(session sqlx.Session) AssetsModel
		// FindAllByIds returns the assets with the given ids.
		FindAllByIds(ctx context.Context, ids []string) ([]*Assets, error)
		// InsertIgnoreConflict inserts an asset, ignoring the (app_id, sha256)
		// unique conflict so concurrent finalize calls stay idempotent.
		InsertIgnoreConflict(ctx context.Context, data *Assets) error
		// CountOrphanByAppId counts assets of an app no longer referenced by
		// any non-deleted update (neither as launch asset nor regular asset).
		CountOrphanByAppId(ctx context.Context, appId string) (int64, error)
		// FindOrphans returns every asset (across all apps) no longer
		// referenced by any non-deleted update, for the asset-gc job.
		FindOrphans(ctx context.Context) ([]*Assets, error)
	}

	customAssetsModel struct {
		*defaultAssetsModel
	}
)

// NewAssetsModel returns a model for the database table.
func NewAssetsModel(conn sqlx.SqlConn) AssetsModel {
	return &customAssetsModel{
		defaultAssetsModel: newAssetsModel(conn),
	}
}

func (m *customAssetsModel) withSession(session sqlx.Session) AssetsModel {
	return NewAssetsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAssetsModel) FindAllByIds(ctx context.Context, ids []string) ([]*Assets, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := fmt.Sprintf("select %s from %s where id = any($1)", assetsRows, m.table)
	var resp []*Assets
	err := m.conn.QueryRowsCtx(ctx, &resp, query, pq.Array(ids))
	return resp, err
}

func (m *customAssetsModel) InsertIgnoreConflict(ctx context.Context, data *Assets) error {
	query := fmt.Sprintf(
		"insert into %s (%s) values ($1, $2, $3, $4, $5, $6, $7, $8) on conflict (app_id, sha256) do nothing",
		m.table, assetsRowsExpectAutoSet)
	_, err := m.conn.ExecCtx(ctx, query,
		data.Id, data.AppId, data.Sha256, data.Sha256B64url,
		data.SizeBytes, data.ContentType, data.FileExt, data.StorageKey)
	return err
}

func (m *customAssetsModel) CountOrphanByAppId(ctx context.Context, appId string) (int64, error) {
	query := `
		select count(*) from "public"."assets" a
		where a.app_id = $1
		  and not exists (
			select 1 from "public"."update_assets" ua
			join "public"."updates" u on u.id = ua.update_id
			where ua.asset_id = a.id and u.deleted_at is null
		  )
		  and not exists (
			select 1 from "public"."updates" u2
			where u2.launch_asset_id = a.id and u2.deleted_at is null
		  )`
	var count int64
	err := m.conn.QueryRowCtx(ctx, &count, query, appId)
	return count, err
}

func (m *customAssetsModel) FindOrphans(ctx context.Context) ([]*Assets, error) {
	query := fmt.Sprintf(`
		select %s from %s a
		where not exists (
			select 1 from "public"."update_assets" ua
			join "public"."updates" u on u.id = ua.update_id
			where ua.asset_id = a.id and u.deleted_at is null
		)
		  and not exists (
			select 1 from "public"."updates" u2
			where u2.launch_asset_id = a.id and u2.deleted_at is null
		  )`, assetsRows, m.table)
	var resp []*Assets
	err := m.conn.QueryRowsCtx(ctx, &resp, query)
	return resp, err
}
