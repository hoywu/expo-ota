package models

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ RuntimeVersionsModel = (*customRuntimeVersionsModel)(nil)

type (
	// RuntimeVersionsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customRuntimeVersionsModel.
	RuntimeVersionsModel interface {
		runtimeVersionsModel
		withSession(session sqlx.Session) RuntimeVersionsModel
	}

	customRuntimeVersionsModel struct {
		*defaultRuntimeVersionsModel
	}
)

// NewRuntimeVersionsModel returns a model for the database table.
func NewRuntimeVersionsModel(conn sqlx.SqlConn) RuntimeVersionsModel {
	return &customRuntimeVersionsModel{
		defaultRuntimeVersionsModel: newRuntimeVersionsModel(conn),
	}
}

func (m *customRuntimeVersionsModel) withSession(session sqlx.Session) RuntimeVersionsModel {
	return NewRuntimeVersionsModel(sqlx.NewSqlConnFromSession(session))
}
