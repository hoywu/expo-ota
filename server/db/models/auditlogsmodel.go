package models

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ AuditLogsModel = (*customAuditLogsModel)(nil)

type (
	// AuditLogsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAuditLogsModel.
	AuditLogsModel interface {
		auditLogsModel
		withSession(session sqlx.Session) AuditLogsModel
	}

	customAuditLogsModel struct {
		*defaultAuditLogsModel
	}
)

// NewAuditLogsModel returns a model for the database table.
func NewAuditLogsModel(conn sqlx.SqlConn) AuditLogsModel {
	return &customAuditLogsModel{
		defaultAuditLogsModel: newAuditLogsModel(conn),
	}
}

func (m *customAuditLogsModel) withSession(session sqlx.Session) AuditLogsModel {
	return NewAuditLogsModel(sqlx.NewSqlConnFromSession(session))
}
