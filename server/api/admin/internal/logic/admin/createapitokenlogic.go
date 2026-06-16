// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"math/big"
	"net/http"
	"time"

	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	tokenPrefix       = "ota_pat_"
	tokenRandomLength = 32
	base62Alphabet    = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var (
	errTokenNameEmpty   = httperr.New(http.StatusBadRequest, "name must not be empty")
	errInvalidExpiresAt = httperr.New(http.StatusBadRequest, "expiresAt must be a valid RFC 3339 timestamp in the future")
)

type CreateApiTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateApiTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateApiTokenLogic {
	return &CreateApiTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateApiTokenLogic) CreateApiToken(req *types.CreateTokenReq) (resp *types.CreateTokenResp, err error) {
	if req.Name == "" {
		return nil, errTokenNameEmpty
	}

	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	actorId, err := userIDFromContext(l.ctx)
	if err != nil {
		return nil, err
	}

	var expiresAt sql.NullTime
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil || !t.After(time.Now()) {
			return nil, errInvalidExpiresAt
		}
		expiresAt = sql.NullTime{Time: t.UTC(), Valid: true}
	}

	plaintext, err := newApiToken()
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(plaintext))

	id, err := newUUID()
	if err != nil {
		return nil, err
	}

	token := &models.ApiTokens{
		Id:        id,
		AppId:     app.Id,
		CreatedBy: actorId,
		Name:      req.Name,
		TokenHash: models.ByteaHex(hash[:]),
		Scopes:    []string{"publish"},
		ExpiresAt: expiresAt,
	}
	if _, err := l.svcCtx.ApiTokensModel.Insert(l.ctx, token); err != nil {
		return nil, err
	}

	created, err := l.svcCtx.ApiTokensModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	writeAudit(l.ctx, l.svcCtx, "create_api_token", app.Id, "api_token", id, map[string]any{
		"name": req.Name,
	})

	return &types.CreateTokenResp{
		Id:        created.Id,
		Name:      created.Name,
		Token:     plaintext, // returned exactly once; only the hash is stored
		ExpiresAt: formatNullTime(created.ExpiresAt),
		CreatedAt: formatTime(created.CreatedAt),
	}, nil
}

// newApiToken generates `ota_pat_<32 base62 chars>` (see server/CONTEXT.md).
func newApiToken() (string, error) {
	max := big.NewInt(int64(len(base62Alphabet)))
	buf := make([]byte, tokenRandomLength)
	for i := range buf {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		buf[i] = base62Alphabet[n.Int64()]
	}
	return tokenPrefix + string(buf), nil
}
