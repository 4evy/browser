package extensions

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestInstallVerifiedGitHubZIPAsUnpackedExtension(t *testing.T) {
	root := t.TempDir()
	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const manifestPublicKey = "MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAj/u/XDdjlDyw7gHEtaaasZ9GdG8WOKAyJzXd8HFrDtz2Jcuy7er7MtWvHgNDA0bwpznbI5YdZeV4UfCEsA4SrA5b3MnWTHwA1bgbiDM+L9rrqvcadcKuOlTeN48Q0ijmhHlNFbTzvT9W0zw/GKv8LgXAHggxtmHQ/Z9PP2QNF5O8rUHHSL4AJ6hNcEKSBVSmbbjeVm4gSXDuED5r0nwxvRtupDxGYp8IZpP5KlExqNu1nbkPc+igCTIB6XsqijagzxewUHCdovmkb2JNtskx/PMIEv+TvWIx2BzqGp71gSh/dV7SJ3rClvWd2xj8dtxG8FfAWDTIIi0qZXWn2QhizQIDAQAB"
	const manifestExtensionID = "lfoeajgcchlidpicbabpmckkejpckcfb"
	archiveData := testZIP(t, map[string]string{
		"extension/manifest.json": `{"name":"Example","version":"1.2.3","key":"` +
			manifestPublicKey + `"}`,
		"extension/worker.js": "export {};",
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
	if got := result.ExtensionIDAliases[extensionID]; got != manifestExtensionID {
		t.Fatalf("extension alias = %q", got)
	}
	if _, err := os.Stat(filepath.Join(want, "manifest.json")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallUnpackedExtensionWithoutManifestKeyUsesPathID(t *testing.T) {
	root := t.TempDir()
	const extensionID = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	archiveData := testZIP(t, map[string]string{
		"extension/manifest.json": `{"name":"Path ID","version":"1.2.3"}`,
	})
	digest := sha256.Sum256(archiveData)
	result, err := Install(t.Context(), Options{
		Root: root,
		Catalog: Catalog{ZIP: []GitHubReleaseExtension{{
			ID:           extensionID,
			Name:         "Path ID",
			UpdatePolicy: UpdatePolicyPinned,
			Version:      "1.2.3",
			URL:          "https://example.test/path-id-1.2.3.zip",
			SHA256:       "sha256-" + base64.StdEncoding.EncodeToString(digest[:]),
			ArchiveRoot:  "extension",
			LoadUnpacked: true,
		}}},
		Download: func(_ context.Context, target, _ string) error {
			return os.WriteFile(target, archiveData, 0o600)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(root, unpackedExtensionDir, extensionID)
	resolvedPath, err := filepath.EvalSymlinks(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.ExtensionIDAliases[extensionID], UnpackedExtensionID(resolvedPath); got != want {
		t.Fatalf("extension alias = %q, want path-derived %q", got, want)
	}
}

func TestInstallUnpackedExtensionWithoutManifestKeyResolvesSymlinkForID(t *testing.T) {
	realRoot := t.TempDir()
	root := filepath.Join(t.TempDir(), "extension-root")
	if err := os.Symlink(realRoot, root); err != nil {
		t.Skipf("create symlink: %v", err)
	}
	const extensionID = "dddddddddddddddddddddddddddddddd"
	archiveData := testZIP(t, map[string]string{
		"extension/manifest.json": `{"name":"Symlink Path ID","version":"1.2.3"}`,
	})
	digest := sha256.Sum256(archiveData)
	result, err := Install(t.Context(), Options{
		Root: root,
		Catalog: Catalog{ZIP: []GitHubReleaseExtension{{
			ID:           extensionID,
			Name:         "Symlink Path ID",
			UpdatePolicy: UpdatePolicyPinned,
			Version:      "1.2.3",
			URL:          "https://example.test/symlink-path-id-1.2.3.zip",
			SHA256:       "sha256-" + base64.StdEncoding.EncodeToString(digest[:]),
			ArchiveRoot:  "extension",
			LoadUnpacked: true,
		}}},
		Download: func(_ context.Context, target, _ string) error {
			return os.WriteFile(target, archiveData, 0o600)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(root, unpackedExtensionDir, extensionID)
	if diff := cmp.Diff([]string{wantPath}, result.LoadExtensionPaths); diff != "" {
		t.Fatalf("load extension paths mismatch (-want +got):\n%s", diff)
	}
	resolvedPath, err := filepath.EvalSymlinks(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.ExtensionIDAliases[extensionID], UnpackedExtensionID(resolvedPath); got != want {
		t.Fatalf("extension alias = %q, want resolved-path-derived %q", got, want)
	}
	if lexicalID := UnpackedExtensionID(wantPath); lexicalID == result.ExtensionIDAliases[extensionID] {
		t.Fatalf("extension alias unexpectedly uses lexical symlink path ID %q", lexicalID)
	}
}

func TestInstallUnpackedExtensionRejectsInvalidManifestKey(t *testing.T) {
	archiveData := testZIP(t, map[string]string{
		"extension/manifest.json": `{"name":"Invalid Key","version":"1.2.3","key":"invalid"}`,
	})
	digest := sha256.Sum256(archiveData)
	_, err := Install(t.Context(), Options{
		Root: t.TempDir(),
		Catalog: Catalog{ZIP: []GitHubReleaseExtension{{
			ID:           "ffffffffffffffffffffffffffffffff",
			Name:         "Invalid Key",
			UpdatePolicy: UpdatePolicyPinned,
			Version:      "1.2.3",
			URL:          "https://example.test/invalid-key-1.2.3.zip",
			SHA256:       "sha256-" + base64.StdEncoding.EncodeToString(digest[:]),
			ArchiveRoot:  "extension",
			LoadUnpacked: true,
		}}},
		Download: func(_ context.Context, target, _ string) error {
			return os.WriteFile(target, archiveData, 0o600)
		},
	})
	if err == nil || !strings.Contains(err.Error(), "manifest public key") {
		t.Fatalf("install error = %v, want invalid manifest public key", err)
	}
}

func TestInstallLeavesManifestV2CompatibilityToBrowser(t *testing.T) {
	// Helium makes Chromium's MV2 disable decision false rather than rewriting
	// manifests: https://github.com/imputnet/helium/blob/208a56bab803ceda8202f8c79dd0e9bfcffe600a/patches/ungoogled-chromium/extensions-manifestv2.patch
	const extensionID = "cccccccccccccccccccccccccccccccc"
	const manifest = `{"name":"MV2 example","version":"1.2.3","manifest_version":2}`
	archiveData := testZIP(t, map[string]string{
		"extension/manifest.json": manifest,
	})
	digest := sha256.Sum256(archiveData)
	root := t.TempDir()
	_, err := Install(t.Context(), Options{
		Root: root,
		Catalog: Catalog{ZIP: []ZIPExtension{{
			ID:           extensionID,
			Name:         "MV2 example",
			UpdatePolicy: UpdatePolicyPinned,
			Version:      "1.2.3",
			URL:          "https://example.test/mv2-1.2.3.zip",
			SHA256:       "sha256-" + base64.StdEncoding.EncodeToString(digest[:]),
			ArchiveRoot:  "extension",
			LoadUnpacked: true,
		}}},
		Download: func(_ context.Context, target, _ string) error {
			return os.WriteFile(target, archiveData, 0o600)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(
		root,
		unpackedExtensionDir,
		extensionID,
		manifestFilename,
	))
	if err != nil {
		t.Fatal(err)
	}
	var installed struct {
		ManifestVersion int `json:"manifest_version"`
	}
	if err := json.Unmarshal(data, &installed); err != nil {
		t.Fatal(err)
	}
	if installed.ManifestVersion != 2 {
		t.Fatalf("manifest_version = %d, want 2", installed.ManifestVersion)
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

func TestReplaceDirectoryRemovesStaleFiles(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "extension")
	source := filepath.Join(parent, "staged")
	for _, directory := range []string{target, source} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(target, "stale"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "current"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := replaceDirectory(source, target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "current")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "stale")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale file survived directory replacement: %v", err)
	}
}

func TestReplaceDirectoryRestoresTargetWhenStagedRenameFails(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "extension")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(target, "keep")
	if err := os.WriteFile(marker, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := replaceDirectory(filepath.Join(parent, "missing"), target)
	if err == nil {
		t.Fatal("missing staged directory was accepted")
	}
	if data, readErr := os.ReadFile(marker); readErr != nil || string(data) != "old" {
		t.Fatalf("previous directory was not restored: data = %q, error = %v", data, readErr)
	}
}

func TestExtensionIDFromManifestKeyHashesDecodedBytesWithoutX509Parsing(t *testing.T) {
	keyBytes := []byte("Chromium treats this as opaque public-key bytes")
	encoded := base64.StdEncoding.EncodeToString(keyBytes)
	got, err := extensionIDFromManifestKey(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if want := extensionIDFromBytes(keyBytes); got != want {
		t.Fatalf("manifest key ID = %q, want %q", got, want)
	}

	pem := "-----BEGIN PUBLIC KEY-----\n" + encoded + "\n-----END PUBLIC KEY-----"
	gotPEM, err := extensionIDFromManifestKey(pem)
	if err != nil {
		t.Fatal(err)
	}
	if gotPEM != got {
		t.Fatalf("PEM manifest key ID = %q, want %q", gotPEM, got)
	}
}

func TestManifestKeyWhitespaceMatchesChromiumStrictDecode(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("opaque key bytes"))
	validPEM := "-----BEGIN PUBLIC KEY-----\n" + encoded +
		"\n-----END PUBLIC KEY-----"
	if _, err := extensionIDFromManifestKey(validPEM); err != nil {
		t.Fatalf("newline-formatted PEM: %v", err)
	}
	for name, key := range map[string]string{
		"raw newline": encoded[:4] + "\n" + encoded[4:],
		"PEM space": "-----BEGIN PUBLIC KEY----- " + encoded +
			" -----END PUBLIC KEY-----",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := extensionIDFromManifestKey(key); err == nil {
				t.Fatal("expected whitespace to be rejected")
			}
		})
	}
}

func TestValidateUnpackedManifestRejectsNullPublicKey(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(directory, manifestFilename),
		[]byte(`{"name":"Invalid Key","version":"1.2.3","key":null}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := validateUnpackedManifest(directory, "invalid key", "1.2.3"); err == nil {
		t.Fatal("expected null manifest key to fail")
	}
}

func TestInstallPinnedGitExtensionWithoutHTTPDownloader(t *testing.T) {
	root := t.TempDir()
	const extensionID = "cccccccccccccccccccccccccccccccc"
	const commit = "0123456789abcdef0123456789abcdef01234567"
	extension := GitExtension{
		ID:           extensionID,
		Name:         "Pinned GitLab extension",
		Provider:     GitProviderGitLab,
		Repository:   "group/team/extension",
		UpdatePolicy: UpdatePolicyPinned,
		Ref:          "v1.2.3",
		Commit:       commit,
		Subdirectory: "dist",
		LoadUnpacked: true,
	}
	result, err := Install(t.Context(), Options{
		Root:    root,
		Catalog: Catalog{Git: []GitExtension{extension}},
		CheckoutGit: func(
			_ context.Context,
			destination string,
			got GitExtension,
		) (GitRevision, error) {
			if got != extension {
				t.Fatalf("Git extension = %#v, want %#v", got, extension)
			}
			return GitRevision{Commit: commit}, os.WriteFile(
				filepath.Join(destination, manifestFilename),
				[]byte(`{"name":"GitLab extension","version":"1.2.3"}`),
				0o600,
			)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(root, unpackedExtensionDir, extensionID)
	if diff := cmp.Diff([]string{wantPath}, result.LoadExtensionPaths); diff != "" {
		t.Fatalf("load extension paths mismatch (-want +got):\n%s", diff)
	}
	if _, err := os.Stat(filepath.Join(wantPath, manifestFilename)); err != nil {
		t.Fatal(err)
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

func TestChromeStoreCRXDownloadURLUsesNestedUpdateQuery(t *testing.T) {
	const extensionID = "abcdefghijklmnopabcdefghijklmnop"
	const chromeVersion = "152.0.7971.0"
	rawURL, err := ChromeStoreCRXDownloadURLForVersion(
		"https://clients2.google.com/service/update2/crx",
		extensionID,
		chromeVersion,
	)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	for key, want := range map[string]string{
		chromeStoreResponseKey:       chromeStoreResponseRedirect,
		chromeStoreProductVersionKey: chromeVersion,
		chromeStoreAcceptFormatKey:   "crx3",
		chromeStoreOSKey:             chromiumUpdateOS(),
		chromeStoreArchitectureKey:   chromiumUpdateArchitecture(),
		chromeStoreProductKey:        chromeStoreProduct,
		chromeStoreExtensionKey:      "id=" + extensionID + chromeStoreInstallSuffix,
	} {
		if got := query.Get(key); got != want {
			t.Errorf("query %s = %q, want %q", key, got, want)
		}
	}
}

func TestInstallWritesAbsoluteExternalCRXPath(t *testing.T) {
	workingDir := t.TempDir()
	t.Chdir(workingDir)
	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, err := Install(t.Context(), Options{
		Root:         "state",
		ExternalDirs: []string{"external"},
		Catalog: Catalog{CRX: []DownloadedExtension{{
			ID: extensionID, Name: "External", Version: "1.2.3",
			URL:    "https://example.test/external.crx",
			SHA256: "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		}}},
		Download: func(_ context.Context, target, _ string) error {
			return os.WriteFile(target, []byte("test CRX"), 0o600)
		},
		Verify:    func(_, _ string) error { return nil },
		VerifyCRX: func(_, _ string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("external", extensionID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var definition map[string]string
	if err := json.Unmarshal(data, &definition); err != nil {
		t.Fatal(err)
	}
	if path := definition[externalCRXPathKey]; !filepath.IsAbs(path) {
		t.Fatalf("external_crx = %q, want absolute path", path)
	}
}

func TestInstallRemovesOnlyPreviouslyManagedExternalDefinitions(t *testing.T) {
	root := t.TempDir()
	externalDir := filepath.Join(root, "external")
	const managedID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const unmanagedID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	const modifiedID = "cccccccccccccccccccccccccccccccc"
	options := Options{
		Root:         filepath.Join(root, "state"),
		ExternalDirs: []string{externalDir},
		Catalog: Catalog{UpdateURL: []UpdateURLExtension{
			{ID: managedID, Name: "Managed", UpdateURL: "https://example.test/update"},
			{ID: modifiedID, Name: "Modified", UpdateURL: "https://example.test/modified"},
		}},
		Download: func(context.Context, string, string) error { return nil },
	}
	if _, err := Install(t.Context(), options); err != nil {
		t.Fatal(err)
	}
	unmanagedPath := filepath.Join(externalDir, unmanagedID+externalManifestFileExtension)
	if err := os.WriteFile(unmanagedPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	modifiedPath := filepath.Join(externalDir, modifiedID+externalManifestFileExtension)
	if err := os.WriteFile(modifiedPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	options.ExternalDirs = nil
	options.Catalog.UpdateURL = nil
	if _, err := Install(t.Context(), options); err != nil {
		t.Fatal(err)
	}
	managedPath := filepath.Join(externalDir, managedID+externalManifestFileExtension)
	if _, err := os.Stat(managedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("managed definition still exists: %v", err)
	}
	if _, err := os.Stat(unmanagedPath); err != nil {
		t.Fatalf("unmanaged definition was removed: %v", err)
	}
	if data, err := os.ReadFile(modifiedPath); err != nil {
		t.Fatalf("modified managed definition was removed: %v", err)
	} else if string(data) != "{}\n" {
		t.Fatalf("modified managed definition = %q, want preserved", data)
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
	if err := VerifyFileSHA256(path, hex.EncodeToString(digest[:])); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFileSHA256(
		path,
		"sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestUnpackedExtensionIDMatchesChromiumPOSIXVector(t *testing.T) {
	// Vector: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/components/crx_file/id_util_unittest.cc#68
	if runtime.GOOS == "windows" {
		t.Skip("browser supports macOS and Linux path encoding")
	}
	if got, want := UnpackedExtensionID("/path/to/file.ext"), "lnkgfdknojmdambfcanadbhmfjfljobb"; got != want {
		t.Fatalf("unpacked extension ID = %q, want Chromium vector %q", got, want)
	}
}

func TestVerifyCRXIDConvertsMalformedHeaderPanicToError(t *testing.T) {
	// The parser slices from the untrusted header length here:
	// https://github.com/mediabuyerbot/go-crx3/blob/v1.7.0/id.go#L35-L42
	data := make([]byte, 12)
	copy(data, "Cr24")
	binary.LittleEndian.PutUint32(data[4:8], 3)
	binary.LittleEndian.PutUint32(data[8:12], ^uint32(0))
	path := filepath.Join(t.TempDir(), "malformed.crx")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	err := verifyCRXID(path, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err == nil || !strings.Contains(err.Error(), "read CRX extension ID") {
		t.Fatalf("verify malformed CRX error = %v", err)
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
