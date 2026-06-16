package models

import (
	"context"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UpdatesModel = (*customUpdatesModel)(nil)

type (
	// UpdatesFilter narrows FindManyByAppId. Zero values mean "no filter".
	UpdatesFilter struct {
		RuntimeVersionId string
		Platform         string
		Status           string
		CursorId         string // return rows with id < CursorId (uuidv7 is time-ordered)
		Limit            int
	}

	// UpdatesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUpdatesModel.
	UpdatesModel interface {
		updatesModel
		withSession(session sqlx.Session) UpdatesModel
		// FindManyByAppId lists non-deleted updates of an app, newest first.
		FindManyByAppId(ctx context.Context, appId string, filter UpdatesFilter) ([]*Updates, error)
		// InsertWithAssets inserts an update row and its update_assets rows
		// in a single transaction.
		InsertWithAssets(ctx context.Context, data *Updates, assetRows []*UpdateAssets) error
		// Publish flips a pending update to published and stamps
		// published_at. Returns the number of affected rows (0 means the
		// update was not in pending state).
		Publish(ctx context.Context, id string) (int64, error)
		// SoftDelete stamps deleted_at on an update.
		SoftDelete(ctx context.Context, id string) error
		// PublishedRank returns the 1-based rank of a published update within
		// its (app, runtime_version, platform) stream ordered by published_at
		// DESC, counting only published, non-deleted updates.
		PublishedRank(ctx context.Context, id string) (int64, error)
		// SoftDeleteBeyondRank soft-deletes every published update whose rank
		// in its stream exceeds keepLatestN, returning the deleted ids.
		SoftDeleteBeyondRank(ctx context.Context, appId string, keepLatestN int) ([]string, error)
	}

	customUpdatesModel struct {
		*defaultUpdatesModel
	}
)

// NewUpdatesModel returns a model for the database table.
func NewUpdatesModel(conn sqlx.SqlConn) UpdatesModel {
	return &customUpdatesModel{
		defaultUpdatesModel: newUpdatesModel(conn),
	}
}

func (m *customUpdatesModel) withSession(session sqlx.Session) UpdatesModel {
	return NewUpdatesModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customUpdatesModel) FindManyByAppId(ctx context.Context, appId string, filter UpdatesFilter) ([]*Updates, error) {
	conds := []string{"app_id = $1", "deleted_at is null"}
	args := []any{appId}

	addCond := func(cond, value string) {
		args = append(args, value)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if filter.RuntimeVersionId != "" {
		addCond("runtime_version_id = $%d", filter.RuntimeVersionId)
	}
	if filter.Platform != "" {
		addCond("platform = $%d", filter.Platform)
	}
	if filter.Status != "" {
		addCond("status = $%d", filter.Status)
	}
	if filter.CursorId != "" {
		addCond("id < $%d", filter.CursorId)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}

	query := fmt.Sprintf("select %s from %s where %s order by id desc limit %d",
		updatesRows, m.table, strings.Join(conds, " and "), limit)
	var resp []*Updates
	err := m.conn.QueryRowsCtx(ctx, &resp, query, args...)
	return resp, err
}

func (m *customUpdatesModel) InsertWithAssets(ctx context.Context, data *Updates, assetRows []*UpdateAssets) error {
	return m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := m.withSession(session).Insert(ctx, data); err != nil {
			return err
		}
		uaModel := NewUpdateAssetsModel(sqlx.NewSqlConnFromSession(session))
		for _, row := range assetRows {
			if _, err := uaModel.Insert(ctx, row); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *customUpdatesModel) Publish(ctx context.Context, id string) (int64, error) {
	query := fmt.Sprintf(
		"update %s set status = 'published', published_at = now() where id = $1 and status = 'pending' and deleted_at is null",
		m.table)
	result, err := m.conn.ExecCtx(ctx, query, id)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (m *customUpdatesModel) SoftDelete(ctx context.Context, id string) error {
	query := fmt.Sprintf("update %s set deleted_at = now() where id = $1 and deleted_at is null", m.table)
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}

func (m *customUpdatesModel) PublishedRank(ctx context.Context, id string) (int64, error) {
	query := fmt.Sprintf(`
		with target as (
			select app_id, runtime_version_id, platform, published_at
			from %[1]s where id = $1
		)
		select count(*) from %[1]s u, target t
		where u.app_id = t.app_id
		  and u.runtime_version_id = t.runtime_version_id
		  and u.platform = t.platform
		  and u.status = 'published'
		  and u.deleted_at is null
		  and u.published_at >= t.published_at`, m.table)
	var rank int64
	err := m.conn.QueryRowCtx(ctx, &rank, query, id)
	return rank, err
}

func (m *customUpdatesModel) SoftDeleteBeyondRank(ctx context.Context, appId string, keepLatestN int) ([]string, error) {
	query := fmt.Sprintf(`
		with ranked as (
			select id, row_number() over (
				partition by runtime_version_id, platform
				order by published_at desc
			) as rnk
			from %[1]s
			where app_id = $1 and status = 'published' and deleted_at is null
		)
		update %[1]s set deleted_at = now()
		where id in (select id from ranked where rnk > $2)
		returning id`, m.table)
	var ids []string
	err := m.conn.QueryRowsCtx(ctx, &ids, query, appId, keepLatestN)
	return ids, err
}
