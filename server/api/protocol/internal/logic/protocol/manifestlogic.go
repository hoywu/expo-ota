// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package protocol

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/hoywu/expo-ota/server/api/protocol/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/protocol/internal/svc"
	"github.com/hoywu/expo-ota/server/api/protocol/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"

	"github.com/zeromicro/go-zero/core/logx"
)

// noUpdateAvailableDirective is the only directive served by the MVP (§5.2).
var noUpdateAvailableDirective = []byte(`{"type":"noUpdateAvailable"}`)

var (
	errBadProtocolVersion = httperr.New(http.StatusBadRequest, "expo-protocol-version must be 1")
	errBadPlatform        = httperr.New(http.StatusBadRequest, "expo-platform must be ios or android")
	errMissingRuntime     = httperr.New(http.StatusBadRequest, "expo-runtime-version is required")
	errSigningUnavailable = httperr.New(http.StatusInternalServerError, "code signing key is unavailable")
)

type ManifestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewManifestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ManifestLogic {
	return &ManifestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Manifest runs the manifest negotiation algorithm (§5.1) and returns a
// fully-rendered response. Errors are returned only for cases that occur
// before an app is resolved (no app_id to observe); every resolved-app
// outcome is recorded in manifest_requests and returned as a response.
func (l *ManifestLogic) Manifest(req *types.ManifestReq) (*ManifestResponse, error) {
	// 1-2. Validate protocol headers.
	if req.ProtocolVersion != "1" {
		return nil, errBadProtocolVersion
	}
	if req.Platform != "ios" && req.Platform != "android" {
		return nil, errBadPlatform
	}
	if req.RuntimeVersion == "" {
		return nil, errMissingRuntime
	}

	// 3. Resolve the app.
	app, err := findActiveApp(l.ctx, l.svcCtx, req.AppSlug)
	if err != nil {
		return nil, err
	}

	mode := negotiate(req.Accept)
	if mode == acceptNone {
		return l.response(app, req, "not_acceptable", "", http.StatusNotAcceptable, nil, ""), nil
	}

	// 4. Lazily resolve the runtime version.
	rv, err := l.findOrCreateRuntimeVersion(app.Id, req.RuntimeVersion)
	if err != nil {
		return nil, err
	}

	// 5. Find the current published update of the stream.
	update, err := l.svcCtx.UpdatesModel.FindLatestPublished(l.ctx, app.Id, rv.Id, req.Platform)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return l.respondNoUpdate(app, req, mode), nil
		}
		return nil, err
	}

	// 7. Client already runs the latest update -> noUpdateAvailable.
	if req.CurrentUpdateId != "" && req.CurrentUpdateId == update.ManifestUuid {
		return l.respondNoUpdateAvailable(app, req, mode), nil
	}

	// 8-9. Serve the manifest, signing it when the client expects a signature.
	return l.respondManifest(app, req, mode, update)
}

// respondManifest renders the manifest body (the persisted snapshot) in the
// negotiated representation, signing it when configured.
func (l *ManifestLogic) respondManifest(app *models.Apps, req *types.ManifestReq, mode acceptMode, update *models.Updates) (*ManifestResponse, error) {
	manifestBody := []byte(update.ManifestSnapshot)

	sigHeader, err := l.signIfExpected(app.Id, manifestBody, req.ExpectSignature)
	if err != nil {
		return nil, err
	}

	if mode == acceptMultipart {
		body, boundary, err := buildMultipart(manifestBody, sigHeader, nil)
		if err != nil {
			return nil, err
		}
		return l.response(app, req, "update", update.Id, http.StatusOK, body,
			"multipart/mixed; boundary="+boundary, withSignature(sigHeader)), nil
	}

	return l.response(app, req, "update", update.Id, http.StatusOK, manifestBody,
		jsonContentType(req.Accept), withSignature(sigHeader)), nil
}

// respondNoUpdateAvailable serves the noUpdateAvailable directive (§5.2):
// a directive part for multipart, or 204 for single-body clients.
func (l *ManifestLogic) respondNoUpdateAvailable(app *models.Apps, req *types.ManifestReq, mode acceptMode) *ManifestResponse {
	if mode == acceptMultipart {
		body, boundary, err := buildMultipart(nil, "", noUpdateAvailableDirective)
		if err != nil {
			l.Errorf("build noUpdateAvailable multipart failed: %v", err)
			return l.response(app, req, "error", "", http.StatusInternalServerError, nil, "")
		}
		return l.response(app, req, "no_update", "", http.StatusOK, body,
			"multipart/mixed; boundary="+boundary)
	}
	return l.response(app, req, "no_update", "", http.StatusNoContent, nil, "")
}

// respondNoUpdate handles a stream that has no published update. A client
// that reports a current update gets 204 (no-op); a fresh client gets an
// empty multipart (zero parts) or 204 (§5.1 step 6).
func (l *ManifestLogic) respondNoUpdate(app *models.Apps, req *types.ManifestReq, mode acceptMode) *ManifestResponse {
	if req.CurrentUpdateId == "" && mode == acceptMultipart {
		body, boundary, err := buildMultipart(nil, "", nil)
		if err != nil {
			l.Errorf("build empty multipart failed: %v", err)
			return l.response(app, req, "error", "", http.StatusInternalServerError, nil, "")
		}
		return l.response(app, req, "no_update", "", http.StatusOK, body,
			"multipart/mixed; boundary="+boundary)
	}
	return l.response(app, req, "no_update", "", http.StatusNoContent, nil, "")
}

// signIfExpected signs the manifest bytes only when the client sends
// expo-expect-signature. When a signature is expected, missing or unusable key
// material is a server misconfiguration and must not degrade to unsigned.
func (l *ManifestLogic) signIfExpected(appId string, manifestBytes []byte, expectSignature string) (string, error) {
	if expectSignature == "" {
		return "", nil
	}

	key, err := l.svcCtx.CodeSigningKeysModel.FindOneByAppId(l.ctx, appId)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			l.Errorf("expo-expect-signature present but no signing key configured for app %s", appId)
			return "", errSigningUnavailable
		}
		return "", err
	}
	if !key.Enabled || key.DisabledAt.Valid {
		l.Errorf("expo-expect-signature present but signing key %s is disabled", key.KeyId)
		return "", errSigningUnavailable
	}
	if key.Algorithm != signingKeyAlgorithm {
		l.Errorf("expo-expect-signature present but signing key %s uses unsupported algorithm %s", key.KeyId, key.Algorithm)
		return "", errSigningUnavailable
	}

	ciphertext := []byte(key.EncryptedPrivateKey)
	plain, err := decryptPrivateKeyPem(l.svcCtx.SigningEncryptionKey, ciphertext)
	if err != nil {
		l.Errorf("signing key %s enabled but private key unusable: %v", key.KeyId, err)
		return "", errSigningUnavailable
	}
	priv, err := parseRsaPrivateKey(plain)
	if err != nil {
		l.Errorf("signing key %s private key parse failed: %v", key.KeyId, err)
		return "", errSigningUnavailable
	}
	sig, err := signManifest(priv, manifestBytes)
	if err != nil {
		return "", err
	}
	return signatureHeader(sig, key.KeyId), nil
}

// findOrCreateRuntimeVersion resolves the runtime version, lazily creating
// it the first time it is seen (§5.1 step 4), tolerating concurrent creates.
func (l *ManifestLogic) findOrCreateRuntimeVersion(appId, version string) (*models.RuntimeVersions, error) {
	rv, err := l.svcCtx.RuntimeVersionsModel.FindOneByAppIdVersion(l.ctx, appId, version)
	if err == nil {
		return rv, nil
	}
	if !errors.Is(err, models.ErrNotFound) {
		return nil, err
	}

	id, err := newUUID()
	if err != nil {
		return nil, err
	}
	if _, err := l.svcCtx.RuntimeVersionsModel.Insert(l.ctx, &models.RuntimeVersions{
		Id:      id,
		AppId:   appId,
		Version: version,
	}); err != nil {
		// A concurrent request may have created it first.
		if rv, ferr := l.svcCtx.RuntimeVersionsModel.FindOneByAppIdVersion(l.ctx, appId, version); ferr == nil {
			return rv, nil
		}
		return nil, err
	}
	return l.svcCtx.RuntimeVersionsModel.FindOne(l.ctx, id)
}

// response builds a ManifestResponse, records the manifest_requests row, and
// applies the common manifest headers plus any extra headers.
func (l *ManifestLogic) response(app *models.Apps, req *types.ManifestReq, result, servedUpdateId string, status int, body []byte, contentType string, extra ...map[string]string) *ManifestResponse {
	header := commonManifestHeader()
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	for _, m := range extra {
		for k, v := range m {
			if v != "" {
				header.Set(k, v)
			}
		}
	}

	l.recordManifestRequest(app.Id, req, result, servedUpdateId, status)

	return &ManifestResponse{
		StatusCode: status,
		Header:     header,
		Body:       body,
	}
}

// recordManifestRequest persists one manifest_requests observation row
// (§10.1). Recording failures must not fail the request.
func (l *ManifestLogic) recordManifestRequest(appId string, req *types.ManifestReq, result, servedUpdateId string, status int) {
	row := &models.ManifestRequests{
		AppId:          appId,
		OccurredAt:     time.Now(),
		RuntimeVersion: req.RuntimeVersion,
		Platform:       req.Platform,
		DeviceId:       nullString(req.DeviceId),
		ServedUpdateId: nullString(servedUpdateId),
		Result:         result,
		HttpStatus:     int64(status),
	}
	if _, err := l.svcCtx.ManifestRequestsModel.Insert(l.ctx, row); err != nil {
		l.Errorf("record manifest_request failed: %v", err)
	}
}

// withSignature returns the expo-signature header map (empty when unsigned).
func withSignature(sig string) map[string]string {
	if sig == "" {
		return nil
	}
	return map[string]string{"expo-signature": sig}
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
