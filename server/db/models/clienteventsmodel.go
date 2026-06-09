package models

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ ClientEventsModel = (*customClientEventsModel)(nil)

type (
	// ClientEventsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customClientEventsModel.
	ClientEventsModel interface {
		clientEventsModel
		withSession(session sqlx.Session) ClientEventsModel
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
