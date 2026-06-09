package models

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ UpdatesModel = (*customUpdatesModel)(nil)

type (
	// UpdatesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUpdatesModel.
	UpdatesModel interface {
		updatesModel
		withSession(session sqlx.Session) UpdatesModel
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
