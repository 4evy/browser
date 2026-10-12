package zen

import (
	"bytes"
	"context"
	"crypto/sha256"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/4evy/tweezle/addons/firefox"
	jsonutil "github.com/4evy/tweezle/internal/json"
	"github.com/carlmjohnson/requests"
	"github.com/iancoleman/orderedmap"
)

func download(ctx context.Context, address string) ([]byte, error) {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("zen Mod asset requires HTTPS: %q", address)
	}
	client := http.Client{Timeout: 30 * time.Second, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if request.URL.Scheme != "https" {
			return errors.New("zen Mod asset redirected away from HTTPS")
		}
		if len(via) >= 10 {
			return errors.New("too many Zen Mod redirects")
		}
		return nil
	}}
	const limit = 8 << 20
	var data []byte
	err = requests.URL(address).Client(&client).
		CheckStatus(http.StatusOK).
		Handle(func(response *http.Response) error {
			var err error
			data, err = io.ReadAll(io.LimitReader(response.Body, limit+1))
			return err
		}).Fetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("download Zen Mod asset %s: %w", address, err)
	}
	if len(data) > limit {
		return nil, errors.New("zen Mod asset exceeds 8 MiB")
	}
	return data, nil
}

func (config Config) mods(profile firefox.Profile) ([]firefox.ProfileFile, map[string]any, error) {
	ctx := profile.Context
	if ctx == nil {
		ctx = context.Background()
	}
	path := filepath.Join(profile.Directory, "zen-themes.json")
	installed, err := loadMods(path)
	if err != nil {
		return nil, nil, err
	}
	manifests := installed.Values()
	var files []firefox.ProfileFile
	styles := map[string][]byte{}
	preferences := map[string]any{}
	for _, id := range slices.Sorted(maps.Keys(config.Mods)) {
		configured := config.Mods[id]
		modPreferences := map[string]any{}
		manifest, _ := manifests[id].(map[string]any)
		refresh := manifest == nil || (configured.Revision != "" && manifest["tweezleRevision"] != configured.Revision)
		if refresh {
			address := "https://zen-browser.github.io/theme-store/themes/" + id + "/theme.json"
			if configured.Revision != "" {
				address = "https://raw.githubusercontent.com/zen-browser/theme-store/" + configured.Revision + "/themes/" + id + "/theme.json"
			}
			data, err := download(ctx, address)
			if err != nil {
				return nil, nil, err
			}
			var updated map[string]any
			if err := json.Unmarshal(data, &updated); err != nil {
				return nil, nil, err
			}
			if updated["id"] != id {
				return nil, nil, fmt.Errorf("zen Mod manifest ID does not match %s", id)
			}
			if oldEnabled, ok := manifest["enabled"].(bool); ok {
				updated["enabled"] = oldEnabled
			}
			manifest = updated
			if configured.Revision != "" {
				manifest["tweezleRevision"] = configured.Revision
			}
		}
		if configured.Enabled != nil {
			manifest["enabled"] = *configured.Enabled
		}
		root := filepath.Join(profile.Directory, "chrome", "zen-themes", id)
		var preferenceData []byte
		for _, asset := range []struct{ field, file string }{{"style", "chrome.css"}, {"readme", "readme.md"}, {"preferences", "preferences.json"}} {
			address, _ := manifest[asset.field].(string)
			if address == "" {
				if asset.field == "style" {
					return nil, nil, fmt.Errorf("zen Mod %s has no stylesheet URL", id)
				}
				continue
			}
			destination := filepath.Join(root, asset.file)
			data, err := os.ReadFile(destination)
			if refresh || errors.Is(err, os.ErrNotExist) {
				if configured.Revision != "" {
					address = strings.Replace(address, "https://raw.githubusercontent.com/zen-browser/theme-store/main/", "https://raw.githubusercontent.com/zen-browser/theme-store/"+configured.Revision+"/", 1)
					manifest[asset.field] = address
				}
				data, err = download(ctx, address)
				if err != nil {
					return nil, nil, err
				}
				files = append(files, firefox.ProfileFile{Path: destination, Data: data})
			} else if err != nil {
				return nil, nil, err
			}
			if asset.field == "style" && configured.StyleSHA256 != "" {
				if !strings.EqualFold(fmt.Sprintf("%x", sha256.Sum256(data)), configured.StyleSHA256) {
					return nil, nil, fmt.Errorf("zen Mod %s stylesheet SHA-256 mismatch", id)
				}
			}
			if asset.field == "style" {
				styles[id] = data
			}
			if asset.field == "preferences" {
				preferenceData = data
			}
		}
		var definitions []struct {
			Property string `json:"property"`
			Type     string `json:"type"`
			Default  any    `json:"defaultValue"`
		}
		if len(preferenceData) > 0 {
			if err := json.Unmarshal(preferenceData, &definitions); err != nil {
				return nil, nil, err
			}
		}
		types := map[string]string{}
		for _, definition := range definitions {
			types[definition.Property] = definition.Type
			if refresh && definition.Default != nil {
				if _, existing := profile.Preferences[definition.Property]; !existing {
					if definition.Type == "checkbox" {
						if value, ok := definition.Default.(bool); ok {
							modPreferences[definition.Property] = value
						}
					} else {
						modPreferences[definition.Property] = fmt.Sprint(definition.Default)
					}
				}
			}
		}
		for name, value := range configured.Preferences {
			typ, exists := types[name]
			if !exists {
				return nil, nil, fmt.Errorf("Mod %s has no preference %q", id, name)
			}
			_, boolean := value.(bool)
			_, text := value.(string)
			if (typ == "checkbox" && !boolean) || (typ != "checkbox" && !text) {
				return nil, nil, fmt.Errorf("Mod preference %s has the wrong type", name)
			}
			modPreferences[name] = value
		}
		for name, value := range modPreferences {
			if previous, exists := preferences[name]; exists && previous != value {
				return nil, nil, fmt.Errorf("mods configure conflicting values for %s", name)
			}
			preferences[name] = value
		}
		installed.Set(id, manifest)
	}
	data, err := json.Marshal(installed, json.Deterministic(true))
	if err != nil {
		return nil, nil, err
	}
	files = append(files, firefox.ProfileFile{Path: path, Data: data})
	// Zen reads the aggregate sheet on startup and rebuilds it only on updates
	// Rebuild every enabled Mod in native insertion order, including unmanaged
	// Mods, so installing or toggling one does not remove the others' CSS
	var combined strings.Builder
	combined.WriteString("/* Generated Zen Mods stylesheet */\n")
	for _, id := range installed.Keys() {
		manifest, ok := manifests[id].(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("invalid installed Zen Mod %s", id)
		}
		if enabled, ok := manifest["enabled"].(bool); ok && !enabled {
			continue
		}
		style, prepared := styles[id]
		if !prepared {
			style, err = os.ReadFile(filepath.Join(profile.Directory, "chrome", "zen-themes", id, "chrome.css"))
			if err != nil {
				return nil, nil, err
			}
		}
		combined.Write(style)
		combined.WriteByte('\n')
	}
	files = append(files, firefox.ProfileFile{Path: filepath.Join(profile.Directory, "chrome", "zen-themes.css"), Data: []byte(combined.String())})
	return files, preferences, nil
}

// Keep strict JSON validation and exact numbers while the ordered map owns
// insertion order and serialization of the native Mod store
func loadMods(path string) (*orderedmap.OrderedMap, error) {
	installed := orderedmap.New()
	installed.SetEscapeHTML(false)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return installed, nil
	}
	if err != nil {
		return nil, err
	}
	manifests, err := jsonutil.Default.Decode[map[string]any](bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if manifests == nil {
		return nil, fmt.Errorf("%s must contain an object", path)
	}
	if err := json.Unmarshal(data, installed); err != nil {
		return nil, err
	}
	for _, id := range installed.Keys() {
		manifest, ok := manifests[id].(map[string]any)
		if !uuid.MatchString(id) || !ok {
			return nil, fmt.Errorf("invalid installed Zen Mod %s", id)
		}
		installed.Set(id, manifest)
	}
	return installed, nil
}
