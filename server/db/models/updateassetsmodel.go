package models

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ UpdateAssetsModel = (*customUpdateAssetsModel)(nil)

type (
	// UpdateAssetsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUpdateAssetsModel.
	UpdateAssetsModel interface {
		updateAssetsModel
		withSession(session sqlx.Session) UpdateAssetsModel
	}

	customUpdateAssetsModel struct {
		*defaultUpdateAssetsModel
	}
)

// NewUpdateAssetsModel returns a model for the database table.
func NewUpdateAssetsModel(conn sqlx.SqlConn) UpdateAssetsModel {
	return &customUpdateAssetsModel{
		defaultUpdateAssetsModel: newUpdateAssetsModel(conn),
	}
}

func (m *customUpdateAssetsModel) withSession(session sqlx.Session) UpdateAssetsModel {
	return NewUpdateAssetsModel(sqlx.NewSqlConnFromSession(session))
}
