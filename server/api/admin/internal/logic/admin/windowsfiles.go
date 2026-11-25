package admin

import (
	"io/fs"
	"net/http"
	"regexp"
	"strings"

	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
)

var windowsInvalidName = regexp.MustCompile("(?i)[<>:\"\\\\|?*\\x00-\\x1f]|[. ]$|^(con|prn|aux|nul|conin\\$|conout\\$|(?:com|lpt)[1-9¹²³])(?:\\.|$)")

// windowsFiles maps each exported path to a content key. Identical HTML pages,
// for example, can occupy several paths while sharing a single asset row.
func windowsEntry(metadata map[string]any, assets []types.PlanAssetItem) (string, error) {
	files, ok := metadata["windowsFiles"].(map[string]any)
	if !ok || len(files) == 0 {
		return "", httperr.New(http.StatusBadRequest, "missing windowsFiles")
	}
	keys := make(map[string]bool, len(assets))
	for _, asset := range assets {
		if _, exists := keys[asset.Key]; exists || asset.Key == "" {
			return "", httperr.New(http.StatusBadRequest, "invalid or duplicate Windows asset key")
		}
		keys[asset.Key] = false
	}
	paths := make(map[string]bool, len(files))
	for name, value := range files {
		key, ok := value.(string)
		if _, exists := keys[key]; !ok || !exists {
			return "", httperr.New(http.StatusBadRequest, "unknown Windows asset for "+name)
		}
		keys[key] = true
		if !fs.ValidPath(name) || name == "." {
			return "", httperr.New(http.StatusBadRequest, "invalid Windows file path: "+name)
		}
		parts := strings.Split(strings.ToLower(name), "/")
		for i, part := range parts {
			if windowsInvalidName.MatchString(part) {
				return "", httperr.New(http.StatusBadRequest, "invalid Windows file name: "+name)
			}
			prefix := strings.Join(parts[:i+1], "/")
			file := i == len(parts)-1
			if wasFile, exists := paths[prefix]; exists && (file || wasFile) {
				return "", httperr.New(http.StatusBadRequest, "conflicting Windows paths: "+name)
			}
			paths[prefix] = file
		}
	}
	entry, ok := files["index.html"].(string)
	if !ok {
		return "", httperr.New(http.StatusBadRequest, "windowsFiles must contain index.html")
	}
	for _, referenced := range keys {
		if !referenced {
			return "", httperr.New(http.StatusBadRequest, "unreferenced Windows asset")
		}
	}
	return entry, nil
}
