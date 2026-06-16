// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"errors"
	"strings"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateUserLogic {
	return &CreateUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateUserLogic) CreateUser(req *types.CreateUserReq) (resp *types.UserItem, err error) {
	// Usernames are normalized to lowercase (§4.3).
	username := strings.ToLower(strings.TrimSpace(req.Username))
	if username == "" {
		return nil, errUsernameEmpty
	}
	if err := validatePassword(req.Password); err != nil {
		return nil, err
	}

	if _, err := l.svcCtx.UsersModel.FindOneByUsername(l.ctx, username); err == nil {
		return nil, errUsernameTaken
	} else if !errors.Is(err, models.ErrNotFound) {
		return nil, err
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	id, err := newUUID()
	if err != nil {
		return nil, err
	}

	user := &models.Users{
		Id:           id,
		Username:     username,
		PasswordHash: hash,
	}
	if _, err := l.svcCtx.UsersModel.Insert(l.ctx, user); err != nil {
		return nil, err
	}

	created, err := l.svcCtx.UsersModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	writeAudit(l.ctx, l.svcCtx, "create_user", "", "user", created.Id, map[string]any{
		"username": created.Username,
	})

	item := userToItem(created)
	return &item, nil
}
