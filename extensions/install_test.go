package extensions

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestInstallVerifiedGitHubZIPAsUnpackedExtension(t *testing.T) {
	root := t.TempDir()
	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	archiveData := testZIP(t, map[string]string{
		"extension/manifest.json": `{"name":"Example","version":"1.2.3"}`,
		"extension/worker.js":     "export {};",
	})
	digest := sha256.Sum256(archiveData)
	checksum := "sha256-" + base64.StdEncoding.EncodeToString(digest[:])

	result, err := Install(t.Context(), Options{
		Root: root,
		Catalog: Catalog{ZIP: []GitHubReleaseExtension{{
			ID:            extensionID,
			Name:          "Example",
			Repository:    "owner/repository",
			AssetTemplate: "extension-{tag}.zip",
			ArchiveRoot:   "extension",
			LoadUnpacked:  true,
		}}},
		Download: func(_ context.Context, target, rawURL string) error {
			if rawURL != "https://example.test/extension-v1.2.3.zip" {
				t.Fatalf("download URL = %q", rawURL)
			}
			return os.WriteFile(target, archiveData, 0o600)
		},
		ResolveLatestRelease: func(
			_ context.Context,
			repository,
			template string,
		) (ReleaseArtifact, error) {
			return ReleaseArtifact{
				Version: "1.2.3",
				URL:     "https://example.test/extension-v1.2.3.zip",
				SHA256:  checksum,
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, unpackedExtensionDir, extensionID)
	if diff := cmp.Diff([]string{want}, result.LoadExtensionPaths); diff != "" {
		t.Fatalf("load extension paths mismatch (-want +got):\n%s", diff)
	}
	if got := result.ExtensionIDAliases[extensionID]; got != UnpackedExtensionID(want) {
		t.Fatalf("extension alias = %q", got)
	}
	if _, err := os.Stat(filepath.Join(want, "manifest.json")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallPinnedZIPUsesConfiguredChecksumAndCachedArchive(t *testing.T) {
	root := t.TempDir()
	const extensionID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	archiveData := testZIP(t, map[string]string{
		"extension/manifest.json": `{"name":"Pinned","version":"2.3.4"}`,
	})
	digest := sha256.Sum256(archiveData)
	checksum := "sha256-" + base64.StdEncoding.EncodeToString(digest[:])
	downloads := 0
	options := Options{
		Root: root,
		Catalog: Catalog{ZIP: []GitHubReleaseExtension{{
			ID:           extensionID,
			Name:         "Pinned",
			UpdatePolicy: "pinned",
			Version:      "2.3.4",
			URL:          "https://example.test/pinned-2.3.4.zip",
			SHA256:       checksum,
			ArchiveRoot:  "extension",
			LoadUnpacked: true,
		}}},
		Download: func(_ context.Context, target, rawURL string) error {
			downloads++
			if rawURL != "https://example.test/pinned-2.3.4.zip" {
				t.Fatalf("download URL = %q", rawURL)
			}
			return os.WriteFile(target, archiveData, 0o600)
		},
	}
	for range 2 {
		if _, err := Install(t.Context(), options); err != nil {
			t.Fatal(err)
		}
	}
	if downloads != 1 {
		t.Fatalf("downloads = %d, want one verified download", downloads)
	}
}

func TestInstallChromeStoreUsesResolvedChromeVersion(t *testing.T) {
	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const chromeVersion = "152.0.7971.0"
	const extensionVersion = "1.2.3"
	updateURL := "https://clients2.google.com/service/update2/crx"
	resolvedURL := "https://clients2.googleusercontent.com/crx/" +
		strings.ToUpper(extensionID) + "_1_2_3.crx"

	assertVersion := func(rawURL string) {
		t.Helper()
		if !strings.Contains(rawURL, "prodversion="+chromeVersion) {
			t.Fatalf("Chrome Store URL = %q", rawURL)
		}
	}
	_, err := Install(t.Context(), Options{
		Root: t.TempDir(),
		Catalog: Catalog{
			ChromeStoreUpdateURL: updateURL,
			ChromeStore: []ChromeStoreExtension{{
				ID:   extensionID,
				Name: "Chrome Store example",
			}},
		},
		ChromeVersion: chromeVersion,
		Resolve: func(_ context.Context, rawURL string) (string, error) {
			assertVersion(rawURL)
			return resolvedURL, nil
		},
		Download: func(_ context.Context, target, rawURL string) error {
			assertVersion(rawURL)
			return os.WriteFile(target, []byte("test CRX"), 0o600)
		},
		VerifyCRX: func(path, id string) error {
			if id != extensionID {
				t.Fatalf("verified extension ID = %q", id)
			}
			if want := extensionID + "-" + extensionVersion + ".crx"; filepath.Base(path) != want {
				t.Fatalf("CRX path = %q, want basename %q", path, want)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestVerifyFileSHA256(t *testing.T) {
	data := []byte("verified")
	digest := sha256.Sum256(data)
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	checksum := "sha256-" + base64.StdEncoding.EncodeToString(digest[:])
	if err := VerifyFileSHA256(path, checksum); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFileSHA256(path, fmt.Sprintf("%x", digest)); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFileSHA256(
		path,
		"sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func testZIP(t *testing.T, files map[string]string) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for name, content := range files {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
