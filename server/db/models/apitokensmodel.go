package models

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ ApiTokensModel = (*customApiTokensModel)(nil)

type (
	// ApiTokensModel is an interface to be customized, add more methods here,
	// and implement the added methods in customApiTokensModel.
	ApiTokensModel interface {
		apiTokensModel
		withSession(session sqlx.Session) ApiTokensModel
	}

	customApiTokensModel struct {
		*defaultApiTokensModel
	}
)

// NewApiTokensModel returns a model for the database table.
func NewApiTokensModel(conn sqlx.SqlConn) ApiTokensModel {
	return &customApiTokensModel{
		defaultApiTokensModel: newApiTokensModel(conn),
	}
}

func (m *customApiTokensModel) withSession(session sqlx.Session) ApiTokensModel {
	return NewApiTokensModel(sqlx.NewSqlConnFromSession(session))
}
