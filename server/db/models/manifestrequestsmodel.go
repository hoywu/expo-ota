package models

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ ManifestRequestsModel = (*customManifestRequestsModel)(nil)

type (
	// ManifestRequestsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customManifestRequestsModel.
	ManifestRequestsModel interface {
		manifestRequestsModel
		withSession(session sqlx.Session) ManifestRequestsModel
	}

	customManifestRequestsModel struct {
		*defaultManifestRequestsModel
	}
)

// NewManifestRequestsModel returns a model for the database table.
func NewManifestRequestsModel(conn sqlx.SqlConn) ManifestRequestsModel {
	return &customManifestRequestsModel{
		defaultManifestRequestsModel: newManifestRequestsModel(conn),
	}
}

func (m *customManifestRequestsModel) withSession(session sqlx.Session) ManifestRequestsModel {
	return NewManifestRequestsModel(sqlx.NewSqlConnFromSession(session))
}
