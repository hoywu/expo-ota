package models

import (
	"context"
	"database/sql"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ClientEventsModel = (*customClientEventsModel)(nil)

type (
	// ClientEventUpdateStats aggregates client events for one update.
	ClientEventUpdateStats struct {
		SucceededDevices int64         `db:"succeeded_devices"`
		FailedDevices    int64         `db:"failed_devices"`
		DurationMinMs    sql.NullInt64 `db:"duration_min_ms"`
		DurationMaxMs    sql.NullInt64 `db:"duration_max_ms"`
		DurationAvgMs    sql.NullInt64 `db:"duration_avg_ms"`
	}

	// ClientEventsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customClientEventsModel.
	ClientEventsModel interface {
		clientEventsModel
		withSession(session sqlx.Session) ClientEventsModel
		// StatsByUpdate aggregates success/failure devices and durations of
		// the given update.
		StatsByUpdate(ctx context.Context, appId, updateId string) (*ClientEventUpdateStats, error)
	}

	customClientEventsModel struct {
		*defaultClientEventsModel
	}
)

// NewClientEventsModel returns a model for the database table.
func NewClientEventsModel(conn sqlx.SqlConn) ClientEventsModel {
	return &customClientEventsModel{
		defaultClientEventsModel: newClientEventsModel(conn),
	}
}

func (m *customClientEventsModel) withSession(session sqlx.Session) ClientEventsModel {
	return NewClientEventsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customClientEventsModel) StatsByUpdate(ctx context.Context, appId, updateId string) (*ClientEventUpdateStats, error) {
	query := `
		select
			count(distinct device_id) filter (where event_type = 'update_succeeded') as succeeded_devices,
			count(distinct device_id) filter (where event_type = 'update_failed')    as failed_devices,
			min(duration_ms) filter (where event_type = 'update_succeeded')          as duration_min_ms,
			max(duration_ms) filter (where event_type = 'update_succeeded')          as duration_max_ms,
			avg(duration_ms) filter (where event_type = 'update_succeeded')::int     as duration_avg_ms
		from "public"."client_events"
		where app_id = $1 and update_id = $2`
	var resp ClientEventUpdateStats
	err := m.conn.QueryRowCtx(ctx, &resp, query, appId, updateId)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
