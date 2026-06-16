package models

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ManifestRequestsModel = (*customManifestRequestsModel)(nil)

type (
	// ManifestRequestsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customManifestRequestsModel.
	ManifestRequestsModel interface {
		manifestRequestsModel
		withSession(session sqlx.Session) ManifestRequestsModel
		// CountDistinctDevices counts distinct devices that were served the
		// given update at the manifest endpoint.
		CountDistinctDevices(ctx context.Context, appId, servedUpdateId string) (int64, error)
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

func (m *customManifestRequestsModel) CountDistinctDevices(ctx context.Context, appId, servedUpdateId string) (int64, error) {
	query := `select count(distinct device_id) from "public"."manifest_requests" where app_id = $1 and served_update_id = $2`
	var count int64
	err := m.conn.QueryRowCtx(ctx, &count, query, appId, servedUpdateId)
	return count, err
}
