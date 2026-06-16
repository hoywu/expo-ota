package models

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ApiTokensModel = (*customApiTokensModel)(nil)

type (
	// ApiTokensModel is an interface to be customized, add more methods here,
	// and implement the added methods in customApiTokensModel.
	ApiTokensModel interface {
		apiTokensModel
		withSession(session sqlx.Session) ApiTokensModel
		// FindAllByAppId returns all tokens of an app, newest first.
		FindAllByAppId(ctx context.Context, appId string) ([]*ApiTokens, error)
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

func (m *customApiTokensModel) FindAllByAppId(ctx context.Context, appId string) ([]*ApiTokens, error) {
	query := fmt.Sprintf("select %s from %s where app_id = $1 order by created_at desc", apiTokensRows, m.table)
	var resp []*ApiTokens
	err := m.conn.QueryRowsCtx(ctx, &resp, query, appId)
	return resp, err
}
