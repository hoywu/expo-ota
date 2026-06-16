package models

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UpdateAssetsModel = (*customUpdateAssetsModel)(nil)

type (
	// UpdateAssetsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUpdateAssetsModel.
	UpdateAssetsModel interface {
		updateAssetsModel
		withSession(session sqlx.Session) UpdateAssetsModel
		// FindAllByUpdateId returns the asset rows of an update in sort order.
		FindAllByUpdateId(ctx context.Context, updateId string) ([]*UpdateAssets, error)
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

func (m *customUpdateAssetsModel) FindAllByUpdateId(ctx context.Context, updateId string) ([]*UpdateAssets, error) {
	query := fmt.Sprintf("select %s from %s where update_id = $1 order by sort_order", updateAssetsRows, m.table)
	var resp []*UpdateAssets
	err := m.conn.QueryRowsCtx(ctx, &resp, query, updateId)
	return resp, err
}
