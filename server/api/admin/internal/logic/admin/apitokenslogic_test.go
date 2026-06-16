package admin

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
	"go.uber.org/mock/gomock"
)

func TestCreateApiTokenReturnsPlaintextOnce(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	var inserted *models.ApiTokens
	m.ApiTokens.EXPECT().Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token *models.ApiTokens) (sql.Result, error) {
			inserted = token
			return nil, nil
		})
	m.ApiTokens.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id string) (*models.ApiTokens, error) {
			return inserted, nil
		})

	resp, err := NewCreateApiTokenLogic(ctxWithUserID("user-1"), svcCtx).CreateApiToken(&types.CreateTokenReq{
		AppSlug: "my-app", Name: "ci",
	})
	if err != nil {
		t.Fatalf("CreateApiToken returned error: %v", err)
	}

	if !strings.HasPrefix(resp.Token, "ota_pat_") || len(resp.Token) != len("ota_pat_")+32 {
		t.Errorf("token format invalid: %q", resp.Token)
	}
	hash := sha256.Sum256([]byte(resp.Token))
	if inserted.TokenHash != models.ByteaHex(hash[:]) {
		t.Error("stored token hash does not match sha256 of plaintext")
	}
	if len(inserted.Scopes) != 1 || inserted.Scopes[0] != "publish" {
		t.Errorf("Scopes = %v", inserted.Scopes)
	}
}

func TestCreateApiTokenRejectsPastExpiry(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)

	_, err := NewCreateApiTokenLogic(ctxWithUserID("user-1"), svcCtx).CreateApiToken(&types.CreateTokenReq{
		AppSlug: "my-app", Name: "ci", ExpiresAt: "2020-01-01T00:00:00Z",
	})
	if !errors.Is(err, errInvalidExpiresAt) {
		t.Errorf("err = %v, want errInvalidExpiresAt", err)
	}
}

func TestRevokeApiTokenWrongApp(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.ApiTokens.EXPECT().FindOne(gomock.Any(), "token-1").
		Return(&models.ApiTokens{Id: "token-1", AppId: "other-app"}, nil)

	_, err := NewRevokeApiTokenLogic(ctxWithUserID("user-1"), svcCtx).RevokeApiToken(&types.TokenIdPath{
		AppSlug: "my-app", TokenId: "token-1",
	})
	if !errors.Is(err, errTokenNotFound) {
		t.Errorf("err = %v, want errTokenNotFound", err)
	}
}

func TestRevokeApiTokenSetsRevokedAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.ApiTokens.EXPECT().FindOne(gomock.Any(), "token-1").
		Return(&models.ApiTokens{Id: "token-1", AppId: "app-1"}, nil)
	m.ApiTokens.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token *models.ApiTokens) error {
			if !token.RevokedAt.Valid {
				t.Error("RevokedAt not set")
			}
			return nil
		})

	if _, err := NewRevokeApiTokenLogic(ctxWithUserID("user-1"), svcCtx).RevokeApiToken(&types.TokenIdPath{
		AppSlug: "my-app", TokenId: "token-1",
	}); err != nil {
		t.Fatalf("RevokeApiToken returned error: %v", err)
	}
}
