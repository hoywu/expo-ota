package admin

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/db/models"
)

var errInvalidSha256 = httperr.New(http.StatusBadRequest, "asset sha256 must be base64url of 32 bytes")

// manifestAsset is one asset entry of a manifest (launchAsset or assets[]).
type manifestAsset struct {
	Key           string `json:"key"`
	Hash          string `json:"hash"`
	ContentType   string `json:"contentType"`
	FileExtension string `json:"fileExtension,omitempty"`
	Url           string `json:"url"`
}

// buildManifest assembles the manifest JSON object (§5.1 field mapping).
// The "id" field is set by the caller after computeManifestUuid.
func buildManifest(runtimeVersion string, createdAt time.Time, launchAsset manifestAsset, assets []manifestAsset, metadata, expoConfig map[string]any) map[string]any {
	if metadata == nil {
		metadata = map[string]any{}
	}
	if assets == nil {
		assets = []manifestAsset{}
	}
	manifest := map[string]any{
		"createdAt":      createdAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		"runtimeVersion": runtimeVersion,
		"launchAsset":    launchAsset,
		"assets":         assets,
		"metadata":       metadata,
		"extra":          map[string]any{"expoClient": expoConfig},
	}
	return manifest
}

// computeManifestUuid derives the persistent manifest UUID (§7.4):
// sha256(canonical manifest JSON) -> first 16 bytes -> UUID layout.
// encoding/json marshals map keys in sorted order, which is our canonical
// form. The manifest must not contain "id" yet.
func computeManifestUuid(manifest map[string]any) (string, error) {
	canonical, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	b := sum[:16]
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// decodeSha256B64url validates and decodes a base64url (unpadded) sha256.
func decodeSha256B64url(s string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(raw) != sha256.Size {
		return nil, errInvalidSha256
	}
	return raw, nil
}

// assetToManifestAsset maps an assets row (+ its manifest key) to the
// manifest asset entry.
func assetToManifestAsset(key string, asset *models.Assets, publicURL string) manifestAsset {
	return manifestAsset{
		Key:           key,
		Hash:          asset.Sha256B64url,
		ContentType:   asset.ContentType,
		FileExtension: asset.FileExt.String,
		Url:           publicURL,
	}
}
