package protocol

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hoywu/expo-ota/server/api/protocol/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
	"go.uber.org/mock/gomock"
)

const testSnapshot = `{"id":"11111111-1111-1111-1111-111111111111","runtimeVersion":"1.0.0"}`

func baseManifestReq() *types.ManifestReq {
	return &types.ManifestReq{
		AppSlug:         "my-app",
		ProtocolVersion: "1",
		Platform:        "ios",
		RuntimeVersion:  "1.0.0",
		Accept:          mediaMultipart,
	}
}

func TestManifestRejectsBadProtocolVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, _ := newTestSvcCtx(ctrl)

	req := baseManifestReq()
	req.ProtocolVersion = "0"
	_, err := NewManifestLogic(context.Background(), svcCtx).Manifest(req)
	if !errors.Is(err, errBadProtocolVersion) {
		t.Errorf("err = %v, want errBadProtocolVersion", err)
	}
}

func TestManifestRejectsBadPlatform(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, _ := newTestSvcCtx(ctrl)

	req := baseManifestReq()
	req.Platform = "web"
	_, err := NewManifestLogic(context.Background(), svcCtx).Manifest(req)
	if !errors.Is(err, errBadPlatform) {
		t.Errorf("err = %v, want errBadPlatform", err)
	}
}

func TestManifestAppNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(nil, models.ErrNotFound)

	_, err := NewManifestLogic(context.Background(), svcCtx).Manifest(baseManifestReq())
	if !errors.Is(err, errAppNotFound) {
		t.Errorf("err = %v, want errAppNotFound", err)
	}
}

func TestManifestNotAcceptable(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)

	req := baseManifestReq()
	req.Accept = "text/html"
	res, err := NewManifestLogic(context.Background(), svcCtx).Manifest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != http.StatusNotAcceptable {
		t.Errorf("status = %d, want 406", res.StatusCode)
	}
}

func TestManifestServesUpdateMultipart(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.RuntimeVersions.EXPECT().FindOneByAppIdVersion(gomock.Any(), "app-1", "1.0.0").Return(newTestRuntimeVersion(), nil)
	m.Updates.EXPECT().FindLatestPublished(gomock.Any(), "app-1", "rv-1", "ios").
		Return(&models.Updates{Id: "u-1", ManifestUuid: "11111111-1111-1111-1111-111111111111", ManifestSnapshot: testSnapshot}, nil)

	res, err := NewManifestLogic(context.Background(), svcCtx).Manifest(baseManifestReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	ct := res.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "multipart/mixed") {
		t.Fatalf("content-type = %q, want multipart/mixed", ct)
	}
	manifest := readManifestPart(t, ct, res.Body)
	if manifest != testSnapshot {
		t.Errorf("manifest part = %q, want %q", manifest, testSnapshot)
	}
}

func TestManifestServesUpdateJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.RuntimeVersions.EXPECT().FindOneByAppIdVersion(gomock.Any(), "app-1", "1.0.0").Return(newTestRuntimeVersion(), nil)
	m.Updates.EXPECT().FindLatestPublished(gomock.Any(), "app-1", "rv-1", "ios").
		Return(&models.Updates{Id: "u-1", ManifestUuid: "abc", ManifestSnapshot: testSnapshot}, nil)

	req := baseManifestReq()
	req.Accept = mediaExpoJSON
	res, err := NewManifestLogic(context.Background(), svcCtx).Manifest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if res.Header.Get("Content-Type") != mediaExpoJSON {
		t.Errorf("content-type = %q, want %q", res.Header.Get("Content-Type"), mediaExpoJSON)
	}
	if string(res.Body) != testSnapshot {
		t.Errorf("body = %q, want %q", res.Body, testSnapshot)
	}
	if res.Header.Get("expo-protocol-version") != "1" {
		t.Errorf("missing expo-protocol-version header")
	}
}

func TestManifestServesPlainJSONContentType(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.RuntimeVersions.EXPECT().FindOneByAppIdVersion(gomock.Any(), "app-1", "1.0.0").Return(newTestRuntimeVersion(), nil)
	m.Updates.EXPECT().FindLatestPublished(gomock.Any(), "app-1", "rv-1", "ios").
		Return(&models.Updates{Id: "u-1", ManifestUuid: "abc", ManifestSnapshot: testSnapshot}, nil)

	// A client that accepts only application/json must not receive a
	// content type outside its Accept set (proactive negotiation, §5.1).
	req := baseManifestReq()
	req.Accept = mediaJSON
	res, err := NewManifestLogic(context.Background(), svcCtx).Manifest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Header.Get("Content-Type") != mediaJSON {
		t.Errorf("content-type = %q, want %q", res.Header.Get("Content-Type"), mediaJSON)
	}
}

func TestManifestNoUpdateAvailableJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	const uuid = "11111111-1111-1111-1111-111111111111"
	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.RuntimeVersions.EXPECT().FindOneByAppIdVersion(gomock.Any(), "app-1", "1.0.0").Return(newTestRuntimeVersion(), nil)
	m.Updates.EXPECT().FindLatestPublished(gomock.Any(), "app-1", "rv-1", "ios").
		Return(&models.Updates{Id: "u-1", ManifestUuid: uuid, ManifestSnapshot: testSnapshot}, nil)

	req := baseManifestReq()
	req.Accept = mediaJSON
	req.CurrentUpdateId = uuid
	res, err := NewManifestLogic(context.Background(), svcCtx).Manifest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", res.StatusCode)
	}
}

func TestManifestNoUpdateAvailableMultipart(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	const uuid = "11111111-1111-1111-1111-111111111111"
	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.RuntimeVersions.EXPECT().FindOneByAppIdVersion(gomock.Any(), "app-1", "1.0.0").Return(newTestRuntimeVersion(), nil)
	m.Updates.EXPECT().FindLatestPublished(gomock.Any(), "app-1", "rv-1", "ios").
		Return(&models.Updates{Id: "u-1", ManifestUuid: uuid, ManifestSnapshot: testSnapshot}, nil)

	req := baseManifestReq()
	req.CurrentUpdateId = uuid
	res, err := NewManifestLogic(context.Background(), svcCtx).Manifest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if !strings.Contains(string(res.Body), "noUpdateAvailable") {
		t.Errorf("body does not contain directive: %s", res.Body)
	}
}

func TestManifestSignsNoUpdateAvailableDirective(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privPem := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	ciphertext := encryptForTest(svcCtx.SigningEncryptionKey, privPem)

	const uuid = "11111111-1111-1111-1111-111111111111"
	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.RuntimeVersions.EXPECT().FindOneByAppIdVersion(gomock.Any(), "app-1", "1.0.0").Return(newTestRuntimeVersion(), nil)
	m.Updates.EXPECT().FindLatestPublished(gomock.Any(), "app-1", "rv-1", "ios").
		Return(&models.Updates{Id: "u-1", ManifestUuid: uuid, ManifestSnapshot: testSnapshot}, nil)
	m.CodeSigningKeys.EXPECT().FindOneByAppId(gomock.Any(), "app-1").
		Return(&models.CodeSigningKeys{KeyId: "main", Algorithm: signingKeyAlgorithm, Enabled: true, EncryptedPrivateKey: ciphertext}, nil)

	req := baseManifestReq()
	req.CurrentUpdateId = uuid
	req.ExpectSignature = `sig, keyid="main", alg="rsa-v1_5-sha256"`
	res, err := NewManifestLogic(context.Background(), svcCtx).Manifest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	directiveBody, directiveHeader := readMultipartPart(t, res.Header.Get("Content-Type"), res.Body, "directive")
	if string(directiveBody) != string(noUpdateAvailableDirective) {
		t.Fatalf("directive body = %q, want %q", directiveBody, noUpdateAvailableDirective)
	}
	sigHeader := directiveHeader.Get("expo-signature")
	if sigHeader == "" {
		t.Fatal("directive expo-signature header missing")
	}
	sig, err := base64.StdEncoding.DecodeString(extractSig(t, sigHeader))
	if err != nil {
		t.Fatalf("signature not base64: %v", err)
	}
	digest := sha256.Sum256(directiveBody)
	if err := rsa.VerifyPKCS1v15(&priv.PublicKey, crypto.SHA256, digest[:], sig); err != nil {
		t.Errorf("directive signature verification failed: %v", err)
	}
}

func TestManifestNoPublishedUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.RuntimeVersions.EXPECT().FindOneByAppIdVersion(gomock.Any(), "app-1", "1.0.0").Return(newTestRuntimeVersion(), nil)
	m.Updates.EXPECT().FindLatestPublished(gomock.Any(), "app-1", "rv-1", "ios").Return(nil, models.ErrNotFound)

	res, err := NewManifestLogic(context.Background(), svcCtx).Manifest(baseManifestReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (empty multipart)", res.StatusCode)
	}
}

func TestManifestLazilyCreatesRuntimeVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.RuntimeVersions.EXPECT().FindOneByAppIdVersion(gomock.Any(), "app-1", "1.0.0").Return(nil, models.ErrNotFound)
	m.RuntimeVersions.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil, nil)
	m.RuntimeVersions.EXPECT().FindOne(gomock.Any(), gomock.Any()).Return(newTestRuntimeVersion(), nil)
	m.Updates.EXPECT().FindLatestPublished(gomock.Any(), "app-1", "rv-1", "ios").Return(nil, models.ErrNotFound)

	req := baseManifestReq()
	req.Accept = mediaJSON
	res, err := NewManifestLogic(context.Background(), svcCtx).Manifest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", res.StatusCode)
	}
}

func TestManifestSignsWhenKeyEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privPem := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	ciphertext := encryptForTest(svcCtx.SigningEncryptionKey, privPem)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.RuntimeVersions.EXPECT().FindOneByAppIdVersion(gomock.Any(), "app-1", "1.0.0").Return(newTestRuntimeVersion(), nil)
	m.Updates.EXPECT().FindLatestPublished(gomock.Any(), "app-1", "rv-1", "ios").
		Return(&models.Updates{Id: "u-1", ManifestUuid: "abc", ManifestSnapshot: testSnapshot}, nil)
	m.CodeSigningKeys.EXPECT().FindOneByAppId(gomock.Any(), "app-1").
		Return(&models.CodeSigningKeys{KeyId: "main", Algorithm: signingKeyAlgorithm, Enabled: true, EncryptedPrivateKey: ciphertext}, nil)

	req := baseManifestReq()
	req.Accept = mediaExpoJSON
	req.ExpectSignature = `sig, keyid="main", alg="rsa-v1_5-sha256"`
	res, err := NewManifestLogic(context.Background(), svcCtx).Manifest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sigHeader := res.Header.Get("expo-signature")
	if sigHeader == "" {
		t.Fatal("expo-signature header missing")
	}
	sigB64 := extractSig(t, sigHeader)
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		t.Fatalf("signature not base64: %v", err)
	}
	digest := sha256.Sum256(res.Body)
	if err := rsa.VerifyPKCS1v15(&priv.PublicKey, crypto.SHA256, digest[:], sig); err != nil {
		t.Errorf("signature verification failed: %v", err)
	}
}

func TestManifestDoesNotSignWithoutExpectSignature(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.RuntimeVersions.EXPECT().FindOneByAppIdVersion(gomock.Any(), "app-1", "1.0.0").Return(newTestRuntimeVersion(), nil)
	m.Updates.EXPECT().FindLatestPublished(gomock.Any(), "app-1", "rv-1", "ios").
		Return(&models.Updates{Id: "u-1", ManifestUuid: "abc", ManifestSnapshot: testSnapshot}, nil)

	req := baseManifestReq()
	req.Accept = mediaExpoJSON
	res, err := NewManifestLogic(context.Background(), svcCtx).Manifest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := res.Header.Get("expo-signature"); got != "" {
		t.Fatalf("expo-signature = %q, want empty when request does not expect signature", got)
	}
}

func TestManifestErrorsWhenExpectedSignatureKeyMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.RuntimeVersions.EXPECT().FindOneByAppIdVersion(gomock.Any(), "app-1", "1.0.0").Return(newTestRuntimeVersion(), nil)
	m.Updates.EXPECT().FindLatestPublished(gomock.Any(), "app-1", "rv-1", "ios").
		Return(&models.Updates{Id: "u-1", ManifestUuid: "abc", ManifestSnapshot: testSnapshot}, nil)
	m.CodeSigningKeys.EXPECT().FindOneByAppId(gomock.Any(), "app-1").Return(nil, models.ErrNotFound)

	req := baseManifestReq()
	req.ExpectSignature = `sig, keyid="main", alg="rsa-v1_5-sha256"`
	_, err := NewManifestLogic(context.Background(), svcCtx).Manifest(req)
	if !errors.Is(err, errSigningUnavailable) {
		t.Fatalf("err = %v, want errSigningUnavailable", err)
	}
}

func TestManifestErrorsWhenExpectedSignaturePrivateKeyMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, m := newTestSvcCtx(ctrl)

	m.Apps.EXPECT().FindOneByAppSlug(gomock.Any(), "my-app").Return(newTestApp(), nil)
	m.RuntimeVersions.EXPECT().FindOneByAppIdVersion(gomock.Any(), "app-1", "1.0.0").Return(newTestRuntimeVersion(), nil)
	m.Updates.EXPECT().FindLatestPublished(gomock.Any(), "app-1", "rv-1", "ios").
		Return(&models.Updates{Id: "u-1", ManifestUuid: "abc", ManifestSnapshot: testSnapshot}, nil)
	m.CodeSigningKeys.EXPECT().FindOneByAppId(gomock.Any(), "app-1").
		Return(&models.CodeSigningKeys{KeyId: "main", Algorithm: signingKeyAlgorithm, Enabled: true, EncryptedPrivateKey: nil}, nil)

	req := baseManifestReq()
	req.ExpectSignature = `sig, keyid="main", alg="rsa-v1_5-sha256"`
	_, err := NewManifestLogic(context.Background(), svcCtx).Manifest(req)
	if !errors.Is(err, errSigningUnavailable) {
		t.Fatalf("err = %v, want errSigningUnavailable", err)
	}
}

var sigRe = regexp.MustCompile(`sig="([^"]*)"`)

func extractSig(t *testing.T, header string) string {
	t.Helper()
	matches := sigRe.FindStringSubmatch(header)
	if len(matches) != 2 {
		t.Fatalf("cannot parse sig from %q", header)
	}
	return matches[1]
}

func readManifestPart(t *testing.T, contentType string, body []byte) string {
	t.Helper()
	data, _ := readMultipartPart(t, contentType, body, "manifest")
	return string(data)
}

func readMultipartPart(t *testing.T, contentType string, body []byte, formName string) ([]byte, http.Header) {
	t.Helper()
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse media type: %v", err)
	}
	mr := multipart.NewReader(strings.NewReader(string(body)), params["boundary"])
	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}
		if part.FormName() == formName {
			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("read part: %v", err)
			}
			return data, http.Header(part.Header)
		}
	}
	t.Fatalf("%s part not found", formName)
	return nil, nil
}
