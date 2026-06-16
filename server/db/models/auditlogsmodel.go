package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AuditLogsModel = (*customAuditLogsModel)(nil)

type (
	// AuditLogsFilter narrows FindManyByAppId. Zero values mean "no filter".
	AuditLogsFilter struct {
		Action      string
		ActorUserId string
		From        time.Time
		To          time.Time
		CursorId    int64 // return rows with id < CursorId
		Limit       int
	}

	// AuditLogsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAuditLogsModel.
	AuditLogsModel interface {
		auditLogsModel
		withSession(session sqlx.Session) AuditLogsModel
		// FindManyByAppId lists audit logs of an app, newest first.
		FindManyByAppId(ctx context.Context, appId string, filter AuditLogsFilter) ([]*AuditLogs, error)
	}

	customAuditLogsModel struct {
		*defaultAuditLogsModel
	}
)

// NewAuditLogsModel returns a model for the database table.
func NewAuditLogsModel(conn sqlx.SqlConn) AuditLogsModel {
	return &customAuditLogsModel{
		defaultAuditLogsModel: newAuditLogsModel(conn),
	}
}

func (m *customAuditLogsModel) withSession(session sqlx.Session) AuditLogsModel {
	return NewAuditLogsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAuditLogsModel) FindManyByAppId(ctx context.Context, appId string, filter AuditLogsFilter) ([]*AuditLogs, error) {
	conds := []string{"app_id = $1"}
	args := []any{appId}

	addCond := func(cond string, value any) {
		args = append(args, value)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if filter.Action != "" {
		addCond("action = $%d", filter.Action)
	}
	if filter.ActorUserId != "" {
		addCond("actor_user_id = $%d", filter.ActorUserId)
	}
	if !filter.From.IsZero() {
		addCond("occurred_at >= $%d", filter.From)
	}
	if !filter.To.IsZero() {
		addCond("occurred_at <= $%d", filter.To)
	}
	if filter.CursorId > 0 {
		addCond("id < $%d", filter.CursorId)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf("select %s from %s where %s order by id desc limit %d",
		auditLogsRows, m.table, strings.Join(conds, " and "), limit)
	var resp []*AuditLogs
	err := m.conn.QueryRowsCtx(ctx, &resp, query, args...)
	return resp, err
}
