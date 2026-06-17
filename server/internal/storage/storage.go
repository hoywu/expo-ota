// Package storage abstracts the asset object store (Tencent Cloud COS).
// Logic layers depend on the Store interface so unit tests can mock it.
package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

var assetKeyPrefix string

// Store is the asset object store used by the upload flow.
type Store interface {
	// PublicURL returns the immutable public download URL for a storage key.
	PublicURL(storageKey string) string
	// PresignPut returns a pre-signed PUT URL (and headers the client must
	// send) for direct upload of a missing asset.
	PresignPut(ctx context.Context, storageKey, contentType string, expires time.Duration) (url string, headers map[string]string, err error)
	// Head returns the object size, or an error if the object is missing.
	Head(ctx context.Context, storageKey string) (sizeBytes int64, err error)
	// Delete removes the object at storageKey. Used by orphan asset GC
	// (§9.2/§9.3); COS DELETE on a missing object is not an error.
	Delete(ctx context.Context, storageKey string) error
}

// SetAssetKeyPrefix configures the optional COS object key prefix.
func SetAssetKeyPrefix(prefix string) {
	assetKeyPrefix = strings.Trim(strings.TrimSpace(prefix), "/")
}

// AssetStorageKey returns the canonical COS object key for an asset:
// {prefix}/apps/{appSlug}/assets/{sha256_b64url}. The apps/... suffix is an
// immutable contract (see IMPLEMENTATION.md §2.1) and must never change.
func AssetStorageKey(appSlug, sha256B64url string) string {
	key := fmt.Sprintf("apps/%s/assets/%s", appSlug, sha256B64url)
	if assetKeyPrefix == "" {
		return key
	}
	return assetKeyPrefix + "/" + key
}
