package admin

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"

	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
)

const (
	// signingKeyAlgorithm is the only supported algorithm (§5.5).
	signingKeyAlgorithm = "rsa-v1_5-sha256"
	// signingEncryptionKeyID identifies the env-provided AES key that
	// encrypted code_signing_keys.encrypted_private_key.
	signingEncryptionKeyID = "env-v1"
)

var (
	errSigningKeyNotFound      = httperr.New(http.StatusNotFound, "signing key not found")
	errSigningKeyNotCooledDown = httperr.New(http.StatusBadRequest,
		"signing key must be disabled before deletion")
	errSigningKeyExists   = httperr.New(http.StatusConflict, "an enabled signing key already exists; disable it first")
	errInvalidPublicKey   = httperr.New(http.StatusBadRequest, "publicKeyPem is not a valid PEM-encoded RSA public key")
	errPrivateKeyRequired = httperr.New(http.StatusBadRequest, "privateKeyPem is required")
	errInvalidPrivateKey  = httperr.New(http.StatusBadRequest, "privateKeyPem is not a valid PEM-encoded RSA private key matching the public key")
	errKeyIdEmpty         = httperr.New(http.StatusBadRequest, "keyId must not be empty")
	errUnsupportedAlg     = httperr.New(http.StatusBadRequest, "algorithm must be rsa-v1_5-sha256")
)

// encryptPrivateKeyPem encrypts a private key PEM with AES-256-GCM; the
// random nonce is prepended to the ciphertext.
func encryptPrivateKeyPem(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// generateRsaKeyPair returns (publicKeyPEM, privateKeyPEM) for a new
// 2048-bit RSA key pair.
func generateRsaKeyPair() ([]byte, []byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	publicDer, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	publicPem := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDer})
	privatePem := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	return publicPem, privatePem, nil
}

func parseRsaPublicKey(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errInvalidPublicKey
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
		return nil, errInvalidPublicKey
	}
	if rsaKey, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return rsaKey, nil
	}
	return nil, errInvalidPublicKey
}

func parseRsaPrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errInvalidPrivateKey
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
	}
	return nil, errInvalidPrivateKey
}

// ensureNoEnabledSigningKey enforces the single-active-key invariant (§5.5).
func ensureNoEnabledSigningKey(ctx context.Context, svcCtx *svc.ServiceContext, appId string) error {
	existing, err := svcCtx.CodeSigningKeysModel.FindOneByAppId(ctx, appId)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil
		}
		return err
	}
	if existing.Enabled {
		return errSigningKeyExists
	}
	return nil
}

// recycleDisabledSigningKeyKeyID allows reusing an old key ID by removing
// a disabled historical row with the same (app_id, key_id).
func recycleDisabledSigningKeyKeyID(ctx context.Context, svcCtx *svc.ServiceContext, appId, keyId string) error {
	existing, err := svcCtx.CodeSigningKeysModel.FindOneByAppIdKeyId(ctx, appId, keyId)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil
		}
		return err
	}
	if existing.Enabled {
		return errSigningKeyExists
	}
	return svcCtx.CodeSigningKeysModel.Delete(ctx, existing.Id)
}

func signingKeyToResp(key *models.CodeSigningKeys) *types.SigningKeyResp {
	return &types.SigningKeyResp{
		KeyId:         key.KeyId,
		Algorithm:     key.Algorithm,
		PublicKeyPem:  key.PublicKeyPem,
		Enabled:       key.Enabled,
		CreatedAt:     formatTime(key.CreatedAt),
		DisabledAt:    formatNullTime(key.DisabledAt),
		HasPrivateKey: len(key.EncryptedPrivateKey) > 0,
	}
}
