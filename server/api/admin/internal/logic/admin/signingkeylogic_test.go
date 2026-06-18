package admin

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
	"go.uber.org/mock/gomock"
)

func TestGenerateSigningKeyStoresEncryptedPrivateKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.CodeSigningKeys.EXPECT().FindOneByAppId(gomock.Any(), "app-1").Return(nil, models.ErrNotFound)
	m.CodeSigningKeys.EXPECT().FindOneByAppIdKeyId(gomock.Any(), "app-1", "main").Return(nil, models.ErrNotFound)
	var inserted *models.CodeSigningKeys
	m.CodeSigningKeys.EXPECT().Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, key *models.CodeSigningKeys) (sql.Result, error) {
			inserted = key
			return nil, nil
		})
	m.CodeSigningKeys.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id string) (*models.CodeSigningKeys, error) {
			return inserted, nil
		})

	resp, err := NewGenerateSigningKeyLogic(ctxWithUserID("user-1"), svcCtx).GenerateSigningKey(&types.GenerateSigningKeyReq{
		AppSlug: "my-app", KeyId: "main",
	})
	if err != nil {
		t.Fatalf("GenerateSigningKey returned error: %v", err)
	}

	if !resp.Enabled || resp.KeyId != "main" || resp.Algorithm != signingKeyAlgorithm {
		t.Errorf("resp = %+v", resp)
	}
	if !resp.HasPrivateKey {
		t.Error("HasPrivateKey = false, want true")
	}
	if !strings.Contains(resp.PublicKeyPem, "PUBLIC KEY") {
		t.Errorf("PublicKeyPem invalid: %q", resp.PublicKeyPem)
	}

	// The stored ciphertext must decrypt back to a PEM private key.
	raw, err := hex.DecodeString(strings.TrimPrefix(inserted.EncryptedPrivateKey, `\x`))
	if err != nil {
		t.Fatalf("EncryptedPrivateKey is not hex bytea: %v", err)
	}
	block, _ := aes.NewCipher(svcCtx.SigningEncryptionKey)
	gcm, _ := cipher.NewGCM(block)
	plaintext, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if !strings.Contains(string(plaintext), "RSA PRIVATE KEY") {
		t.Error("decrypted plaintext is not an RSA private key PEM")
	}
}

func TestGenerateSigningKeyConflictsWithEnabledKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.CodeSigningKeys.EXPECT().FindOneByAppId(gomock.Any(), "app-1").
		Return(&models.CodeSigningKeys{Id: "key-1", Enabled: true}, nil)

	_, err := NewGenerateSigningKeyLogic(ctxWithUserID("user-1"), svcCtx).GenerateSigningKey(&types.GenerateSigningKeyReq{
		AppSlug: "my-app", KeyId: "main",
	})
	if !errors.Is(err, errSigningKeyExists) {
		t.Errorf("err = %v, want errSigningKeyExists", err)
	}
}

func TestImportSigningKeyRejectsMismatchedPair(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.CodeSigningKeys.EXPECT().FindOneByAppId(gomock.Any(), "app-1").Return(nil, models.ErrNotFound)
	m.CodeSigningKeys.EXPECT().FindOneByAppIdKeyId(gomock.Any(), "app-1", "main").Return(nil, models.ErrNotFound)

	publicPem, _, err := generateRsaKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	_, otherPrivatePem, err := generateRsaKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewImportSigningKeyLogic(ctxWithUserID("user-1"), svcCtx).ImportSigningKey(&types.ImportSigningKeyReq{
		AppSlug:       "my-app",
		KeyId:         "main",
		PublicKeyPem:  string(publicPem),
		PrivateKeyPem: string(otherPrivatePem),
	})
	if !errors.Is(err, errInvalidPrivateKey) {
		t.Errorf("err = %v, want errInvalidPrivateKey", err)
	}
}

func TestImportSigningKeyPublicOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.CodeSigningKeys.EXPECT().FindOneByAppId(gomock.Any(), "app-1").Return(nil, models.ErrNotFound)
	m.CodeSigningKeys.EXPECT().FindOneByAppIdKeyId(gomock.Any(), "app-1", "main").Return(nil, models.ErrNotFound)
	var inserted *models.CodeSigningKeys
	m.CodeSigningKeys.EXPECT().Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, key *models.CodeSigningKeys) (sql.Result, error) {
			inserted = key
			return nil, nil
		})
	m.CodeSigningKeys.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id string) (*models.CodeSigningKeys, error) {
			return inserted, nil
		})

	publicPem, _, err := generateRsaKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	resp, err := NewImportSigningKeyLogic(ctxWithUserID("user-1"), svcCtx).ImportSigningKey(&types.ImportSigningKeyReq{
		AppSlug: "my-app", KeyId: "main", PublicKeyPem: string(publicPem),
	})
	if err != nil {
		t.Fatalf("ImportSigningKey returned error: %v", err)
	}
	if resp.HasPrivateKey {
		t.Error("HasPrivateKey = true, want false")
	}
}

func TestDeleteSigningKeyRequiresCooldown(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	cases := []struct {
		name string
		key  *models.CodeSigningKeys
		want error
	}{
		{"enabled", &models.CodeSigningKeys{Id: "k", Enabled: true}, errSigningKeyNotCooledDown},
		{"not disabled", &models.CodeSigningKeys{Id: "k", Enabled: false}, errSigningKeyNotCooledDown},
	}
	for _, tc := range cases {
		m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
		m.CodeSigningKeys.EXPECT().FindOneByAppIdKeyId(gomock.Any(), "app-1", "main").Return(tc.key, nil)

		_, err := NewDeleteSigningKeyLogic(ctxWithUserID("user-1"), svcCtx).DeleteSigningKey(&types.SigningKeyIdPath{
			AppSlug: "my-app", KeyId: "main",
		})
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestDeleteSigningKeyAfterCooldown(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	key := &models.CodeSigningKeys{
		Id: "key-1", Enabled: false,
		DisabledAt: sql.NullTime{Valid: true},
	}
	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.CodeSigningKeys.EXPECT().FindOneByAppIdKeyId(gomock.Any(), "app-1", "main").Return(key, nil)
	m.CodeSigningKeys.EXPECT().Delete(gomock.Any(), "key-1").Return(nil)

	if _, err := NewDeleteSigningKeyLogic(ctxWithUserID("user-1"), svcCtx).DeleteSigningKey(&types.SigningKeyIdPath{
		AppSlug: "my-app", KeyId: "main",
	}); err != nil {
		t.Fatalf("DeleteSigningKey returned error: %v", err)
	}
}

func TestGenerateSigningKeyReusesDisabledKeyID(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	oldKey := &models.CodeSigningKeys{
		Id:         "old-key",
		AppId:      "app-1",
		KeyId:      "main",
		Enabled:    false,
		DisabledAt: sql.NullTime{Valid: true},
	}

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.CodeSigningKeys.EXPECT().FindOneByAppId(gomock.Any(), "app-1").Return(oldKey, nil)
	m.CodeSigningKeys.EXPECT().FindOneByAppIdKeyId(gomock.Any(), "app-1", "main").Return(oldKey, nil)
	m.CodeSigningKeys.EXPECT().Delete(gomock.Any(), "old-key").Return(nil)

	var inserted *models.CodeSigningKeys
	m.CodeSigningKeys.EXPECT().Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, key *models.CodeSigningKeys) (sql.Result, error) {
			inserted = key
			return nil, nil
		})
	m.CodeSigningKeys.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id string) (*models.CodeSigningKeys, error) {
			return inserted, nil
		})

	resp, err := NewGenerateSigningKeyLogic(ctxWithUserID("user-1"), svcCtx).GenerateSigningKey(&types.GenerateSigningKeyReq{
		AppSlug: "my-app", KeyId: "main",
	})
	if err != nil {
		t.Fatalf("GenerateSigningKey returned error: %v", err)
	}
	if resp.KeyId != "main" || !resp.Enabled {
		t.Errorf("resp = %+v", resp)
	}
}

func TestPatchSigningKeyDisable(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newFullTestSvcCtx(ctrl)

	key := &models.CodeSigningKeys{Id: "key-1", AppId: "app-1", Enabled: true}
	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.CodeSigningKeys.EXPECT().FindOneByAppIdKeyId(gomock.Any(), "app-1", "main").Return(key, nil)
	m.CodeSigningKeys.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, k *models.CodeSigningKeys) error {
			if k.Enabled || !k.DisabledAt.Valid {
				t.Errorf("key not disabled correctly: %+v", k)
			}
			return nil
		})

	resp, err := NewPatchSigningKeyLogic(ctxWithUserID("user-1"), svcCtx).PatchSigningKey(&types.PatchSigningKeyReq{
		AppSlug: "my-app", KeyId: "main", Enabled: false,
	})
	if err != nil {
		t.Fatalf("PatchSigningKey returned error: %v", err)
	}
	if resp.Enabled {
		t.Error("resp.Enabled = true, want false")
	}
}
