package storage

import "testing"

func TestAssetStorageKey(t *testing.T) {
	t.Cleanup(func() {
		SetAssetKeyPrefix("")
	})

	SetAssetKeyPrefix("")
	if got, want := AssetStorageKey("my-app", "sha256"), "apps/my-app/assets/sha256"; got != want {
		t.Fatalf("AssetStorageKey() = %q, want %q", got, want)
	}

	SetAssetKeyPrefix("expo-ota")
	if got, want := AssetStorageKey("my-app", "sha256"), "expo-ota/apps/my-app/assets/sha256"; got != want {
		t.Fatalf("AssetStorageKey() = %q, want %q", got, want)
	}

	SetAssetKeyPrefix("/expo-ota/")
	if got, want := AssetStorageKey("my-app", "sha256"), "expo-ota/apps/my-app/assets/sha256"; got != want {
		t.Fatalf("AssetStorageKey() = %q, want %q", got, want)
	}
}
