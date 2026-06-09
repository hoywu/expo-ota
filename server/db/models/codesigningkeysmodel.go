package models

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ CodeSigningKeysModel = (*customCodeSigningKeysModel)(nil)

type (
	// CodeSigningKeysModel is an interface to be customized, add more methods here,
	// and implement the added methods in customCodeSigningKeysModel.
	CodeSigningKeysModel interface {
		codeSigningKeysModel
		withSession(session sqlx.Session) CodeSigningKeysModel
	}

	customCodeSigningKeysModel struct {
		*defaultCodeSigningKeysModel
	}
)

// NewCodeSigningKeysModel returns a model for the database table.
func NewCodeSigningKeysModel(conn sqlx.SqlConn) CodeSigningKeysModel {
	return &customCodeSigningKeysModel{
		defaultCodeSigningKeysModel: newCodeSigningKeysModel(conn),
	}
}

func (m *customCodeSigningKeysModel) withSession(session sqlx.Session) CodeSigningKeysModel {
	return NewCodeSigningKeysModel(sqlx.NewSqlConnFromSession(session))
}
