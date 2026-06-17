package models

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ CodeSigningKeysModel = (*customCodeSigningKeysModel)(nil)

type (
	// CodeSigningKeysModel is an interface to be customized, add more methods here,
	// and implement the added methods in customCodeSigningKeysModel.
	CodeSigningKeysModel interface {
		codeSigningKeysModel
		withSession(session sqlx.Session) CodeSigningKeysModel
		// FindOneByAppId returns the app's signing key (MVP: at most one per
		// app; the latest row wins). Returns ErrNotFound when absent.
		FindOneByAppId(ctx context.Context, appId string) (*CodeSigningKeys, error)
		// ListByAppId returns all signing keys for an app, newest first.
		ListByAppId(ctx context.Context, appId string) ([]*CodeSigningKeys, error)
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

func (m *customCodeSigningKeysModel) FindOneByAppId(ctx context.Context, appId string) (*CodeSigningKeys, error) {
	query := fmt.Sprintf("select %s from %s where app_id = $1 order by created_at desc limit 1", codeSigningKeysRows, m.table)
	var resp CodeSigningKeys
	err := m.conn.QueryRowCtx(ctx, &resp, query, appId)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customCodeSigningKeysModel) ListByAppId(ctx context.Context, appId string) ([]*CodeSigningKeys, error) {
	query := fmt.Sprintf("select %s from %s where app_id = $1 order by created_at desc", codeSigningKeysRows, m.table)
	var resp []*CodeSigningKeys
	err := m.conn.QueryRowsCtx(ctx, &resp, query, appId)
	switch err {
	case nil:
		return resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
