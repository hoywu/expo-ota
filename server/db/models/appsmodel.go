package models

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AppsModel = (*customAppsModel)(nil)

type (
	// AppsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAppsModel.
	AppsModel interface {
		appsModel
		withSession(session sqlx.Session) AppsModel
		// FindAllActive returns all apps that are not soft-deleted, newest first.
		FindAllActive(ctx context.Context) ([]*Apps, error)
	}

	customAppsModel struct {
		*defaultAppsModel
	}
)

// NewAppsModel returns a model for the database table.
func NewAppsModel(conn sqlx.SqlConn) AppsModel {
	return &customAppsModel{
		defaultAppsModel: newAppsModel(conn),
	}
}

func (m *customAppsModel) withSession(session sqlx.Session) AppsModel {
	return NewAppsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAppsModel) FindAllActive(ctx context.Context) ([]*Apps, error) {
	query := fmt.Sprintf("select %s from %s where deleted_at is null order by created_at desc", appsRows, m.table)
	var resp []*Apps
	err := m.conn.QueryRowsCtx(ctx, &resp, query)
	return resp, err
}
