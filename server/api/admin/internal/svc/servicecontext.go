// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"github.com/hoywu/expo-ota/server/api/admin/internal/config"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config     config.Config
	DB         sqlx.SqlConn
	UsersModel models.UsersModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewSqlConn("postgres", c.DataSource)

	return &ServiceContext{
		Config:     c,
		DB:         db,
		UsersModel: models.NewUsersModel(db),
	}
}
