package extensions

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pelletier/go-toml/v2"
)

func TestCatalogExtensionIdentityDecodesFromFlatTOML(t *testing.T) {
	var catalog Catalog
	if err := toml.Unmarshal([]byte(`
[[chrome_store]]
id = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
name = "Store"

[[update_url]]
id = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
name = "Updater"
update_url = "https://example.test/update"

[[crx]]
id = "cccccccccccccccccccccccccccccccc"
name = "CRX"

[[zip]]
id = "dddddddddddddddddddddddddddddddd"
name = "ZIP"

[[git]]
id = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
name = "Git"
`), &catalog); err != nil {
		t.Fatal(err)
	}

	want := Catalog{
		ChromeStore: []ChromeStoreExtension{{
			ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Name: "Store",
		}},
		UpdateURL: []UpdateURLExtension{{
			ID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Name: "Updater",
			UpdateURL: "https://example.test/update",
		}},
		CRX: []DownloadedExtension{{
			ID: "cccccccccccccccccccccccccccccccc", Name: "CRX",
		}},
		ZIP: []ZIPExtension{{
			ID: "dddddddddddddddddddddddddddddddd", Name: "ZIP",
		}},
		Git: []GitExtension{{
			ID: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", Name: "Git",
		}},
	}
	if diff := cmp.Diff(want, catalog); diff != "" {
		t.Fatalf("catalog mismatch (-want +got):\n%s", diff)
	}
}
