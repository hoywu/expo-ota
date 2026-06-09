package models

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ AppsModel = (*customAppsModel)(nil)

type (
	// AppsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAppsModel.
	AppsModel interface {
		appsModel
		withSession(session sqlx.Session) AppsModel
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
