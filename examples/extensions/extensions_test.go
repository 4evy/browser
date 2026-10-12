package extensions_test

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/4evy/tweezle"
	"github.com/4evy/tweezle/addons/chrome"
	"github.com/4evy/tweezle/addons/chromium"
	"github.com/4evy/tweezle/addons/chromium/storage"
	lzstring "github.com/daku10/go-lz-string"
	"github.com/google/go-cmp/cmp"
	"github.com/syndtr/goleveldb/leveldb"
)

//go:embed corpus/reviews.json darkreader/*.json stylus/stylus.json sponsorblock/sponsorblock.json obsidian-clipper/*.json res/res.json ghostery/*.json
var samples embed.FS

type coverage struct {
	DeclaredKeys  []string `json:"declared_keys"`
	DeclaredPaths []string `json:"declared_paths"`
	Container     string   `json:"container"`
}

func collectPaths(value any, prefix string, found map[string]bool) {
	found[prefix] = true
	switch value := value.(type) {
	case map[string]any:
		for key, member := range value {
			token := strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
			collectPaths(member, prefix+"/"+token, found)
		}
	case []any:
		for _, member := range value {
			collectPaths(member, prefix+"/*", found)
		}
	}
}

type review struct {
	Slug             string              `json:"slug"`
	ID               string              `json:"id"`
	Sample           string              `json:"sample"`
	Coverage         map[string]coverage `json:"coverage"`
	PositionalTables []struct {
		Module string   `json:"module"`
		Option string   `json:"option"`
		Fields []string `json:"fields"`
	} `json:"positional_tables"`
	SplitValues *struct {
		PlainSample string `json:"plain_sample"`
		Area        string `json:"area"`
	} `json:"split_values"`
	CompressedTemplates *struct {
		PlainSample string   `json:"plain_sample"`
		Keys        []string `json:"keys"`
	} `json:"compressed_templates"`
}

func checkSplitValues(t *testing.T, item review, area string, actual map[string]any) {
	t.Helper()
	if item.SplitValues == nil || item.SplitValues.Area != area {
		return
	}
	decoded := maps.Clone(actual)
	delete(decoded, "corpus-unrelated-sentinel")
	for key, value := range actual {
		meta, ok := value.(map[string]any)
		if !ok {
			continue
		}
		count, ok := meta["__meta_split_count"].(float64)
		if !ok {
			continue
		}
		var text strings.Builder
		for index := 0; index < int(count); index++ {
			chunkKey := key + "_" + strconv.FormatInt(int64(index), 36)
			chunk, ok := actual[chunkKey].(string)
			if !ok {
				t.Fatalf("missing string chunk %s", chunkKey)
			}
			text.WriteString(chunk)
			delete(decoded, chunkKey)
		}
		if err := json.Unmarshal([]byte(text.String()), &value); err != nil {
			t.Fatal(err)
		}
		decoded[key] = value
	}
	data, err := samples.ReadFile(item.SplitValues.PlainSample)
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string]any
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(expected, decoded); diff != "" {
		t.Errorf("decoded persisted split values (-want +got):\n%s", diff)
	}
}

func TestGhosteryManagedPolicy(t *testing.T) {
	config, err := tweezle.LoadConfig("ghostery/managed.toml", chromium.Addon(chrome.Addon()))
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	if err := tweezle.RenderPolicies(context.Background(), &rendered, config); err != nil {
		t.Fatal(err)
	}
	var actual any
	if err := json.Unmarshal(rendered.Bytes(), &actual); err != nil {
		t.Fatal(err)
	}
	data, err := samples.ReadFile("ghostery/managed.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected any
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("Ghostery managed policy (-want +got):\n%s", diff)
	}
}

func checkCompressedTemplates(t *testing.T, item review, actual map[string]any) {
	t.Helper()
	if item.CompressedTemplates == nil {
		return
	}
	data, err := samples.ReadFile(item.CompressedTemplates.PlainSample)
	if err != nil {
		t.Fatal(err)
	}
	var expected []any
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	if len(expected) != len(item.CompressedTemplates.Keys) {
		t.Fatal("template keys and plain samples have different lengths")
	}
	for index, key := range item.CompressedTemplates.Keys {
		chunks, ok := actual[key].([]any)
		if !ok {
			t.Fatalf("%s is not a native array of compressed chunks", key)
		}
		var joined strings.Builder
		for _, chunk := range chunks {
			text, ok := chunk.(string)
			if !ok {
				t.Fatalf("%s contains a non-string chunk", key)
			}
			joined.WriteString(text)
		}
		plain, err := lzstring.DecompressFromUTF16(utf16.Encode([]rune(joined.String())))
		if err != nil {
			t.Fatalf("decompress persisted %s: %v", key, err)
		}
		var decoded any
		if err := json.Unmarshal([]byte(plain), &decoded); err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(expected[index], decoded); diff != "" {
			t.Errorf("persisted template %s (-want +got):\n%s", key, diff)
		}
	}
}

func TestReviewedConfigurations(t *testing.T) {
	data, err := samples.ReadFile("corpus/reviews.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Reviews []review `json:"reviews"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Reviews) == 0 {
		t.Fatal("no reviewed configurations")
	}
	for _, item := range manifest.Reviews {
		t.Run(item.Slug, func(t *testing.T) {
			data, err := samples.ReadFile(item.Sample)
			if err != nil {
				t.Fatal(err)
			}
			var config storage.Settings
			if err := json.Unmarshal(data, &config); err != nil {
				t.Fatal(err)
			}
			profile := t.TempDir()
			areas := map[string]string{
				"local": "Local Extension Settings",
				"sync":  "Sync Extension Settings",
			}
			expected := map[string]map[string]any{}
			for _, entry := range config.Local {
				if entry.ID != item.ID {
					t.Fatalf("local ID %s differs from review ID %s", entry.ID, item.ID)
				}
				expected["local"] = entry.Values
			}
			for _, entry := range config.Sync {
				if entry.ID != item.ID {
					t.Fatalf("sync ID %s differs from review ID %s", entry.ID, item.ID)
				}
				expected["sync"] = entry.Values
			}
			if len(expected) == 0 {
				t.Fatal("sample has no persistent settings")
			}
			for _, table := range item.PositionalTables {
				module := expected["local"]["RESoptions."+table.Module].(map[string]any)
				option := module[table.Option].(map[string]any)
				rows := option["value"].([]any)
				if len(rows) == 0 {
					t.Errorf("%s.%s has no table rows", table.Module, table.Option)
				}
				last := rows[len(rows)-1].([]any)
				if len(last) != len(table.Fields) {
					t.Errorf("%s.%s sample row has %d cells for %d declared fields", table.Module, table.Option, len(last), len(table.Fields))
				}
			}
			for area, spec := range item.Coverage {
				found := map[string]bool{}
				collectPaths(expected[area], "", found)
				for _, path := range spec.DeclaredPaths {
					if !found[path] {
						t.Errorf("%s omits source-declared nested path %s", area, path)
					}
				}
				values := expected[area]
				if spec.Container != "" {
					var ok bool
					values, ok = values[spec.Container].(map[string]any)
					if !ok {
						t.Fatalf("%s lacks object container %s", area, spec.Container)
					}
				}
				for _, key := range spec.DeclaredKeys {
					if _, exists := values[key]; !exists {
						t.Errorf("%s omits source-declared setting %s", area, key)
					}
				}
			}
			for area := range expected {
				dir := filepath.Join(profile, areas[area], item.ID)
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				db, err := leveldb.OpenFile(dir, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := db.Put([]byte("corpus-unrelated-sentinel"), []byte(`{"keep":true}`), nil); err != nil {
					t.Fatal(err)
				}
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
			}
			for pass := range 2 {
				if err := storage.Apply(context.Background(), storage.ApplyOptions{
					ProfileDir: profile,
					SettingsSource: []storage.SettingsSource{{
						Name: item.Sample, Data: data,
					}},
				}); err != nil {
					t.Fatalf("apply pass %d: %v", pass+1, err)
				}
				for area, values := range expected {
					db, err := leveldb.OpenFile(filepath.Join(profile, areas[area], item.ID), nil)
					if err != nil {
						t.Fatal(err)
					}
					actual := map[string]any{}
					syncBytes := 0
					iterator := db.NewIterator(nil, nil)
					for iterator.Next() {
						if area == "sync" {
							itemBytes := len(iterator.Key()) + len(iterator.Value())
							syncBytes += itemBytes
							if itemBytes > 8192 {
								t.Errorf("persisted sync item %s uses %d bytes, exceeding 8192", iterator.Key(), itemBytes)
							}
						}
						var value any
						if err := json.Unmarshal(iterator.Value(), &value); err != nil {
							t.Fatal(err)
						}
						actual[string(iterator.Key())] = value
					}
					iterator.Release()
					if area == "sync" && (syncBytes > 102400 || len(actual) > 512) {
						t.Errorf("persisted sync area uses %d bytes and %d items", syncBytes, len(actual))
					}
					if err := iterator.Error(); err != nil {
						t.Fatal(err)
					}
					if err := db.Close(); err != nil {
						t.Fatal(err)
					}
					want := map[string]any{"corpus-unrelated-sentinel": map[string]any{"keep": true}}
					maps.Copy(want, values)
					if diff := cmp.Diff(want, actual); diff != "" {
						t.Errorf("%s pass %d (-want +got):\n%s", area, pass+1, diff)
					}
					if area == "sync" {
						checkCompressedTemplates(t, item, actual)
					}
					checkSplitValues(t, item, area, actual)
				}
			}
		})
	}
}
