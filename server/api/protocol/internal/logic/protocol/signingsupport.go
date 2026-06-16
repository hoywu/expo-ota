package protocol

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
)

// signingKeyAlgorithm is the only supported algorithm (§5.5).
const signingKeyAlgorithm = "rsa-v1_5-sha256"

var errNoPrivateKey = errors.New("signing key has no private key material")

// decryptPrivateKeyPem reverses encryptPrivateKeyPem (admin side): the
// AES-256-GCM nonce is prepended to the ciphertext.
func decryptPrivateKeyPem(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errNoPrivateKey
	}
	nonce, payload := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, payload, nil)
}

func parseRsaPrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("invalid private key PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
	}
	return nil, errors.New("private key is not a valid RSA key")
}

// signManifest signs the exact manifest bytes that will be sent on the wire
// with RSA-SHA256 PKCS#1 v1.5 (§5.5) and returns the standard-base64 signature.
func signManifest(privateKey *rsa.PrivateKey, manifestBytes []byte) (string, error) {
	digest := sha256.Sum256(manifestBytes)
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// signatureHeader formats the expo-signature structured-field value (§5.5).
func signatureHeader(sigB64, keyID string) string {
	return fmt.Sprintf(`sig="%s", keyid="%s", alg="%s"`, sigB64, keyID, signingKeyAlgorithm)
}
