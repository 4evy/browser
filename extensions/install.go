package extensions

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/4evy/browser/internal/fileutil"
	crx3 "github.com/mediabuyerbot/go-crx3"
)

// The Chromium-sensitive contracts in this file were audited on 2026-08-23.
// Chromium main (CRX, external providers, APIs):
// https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/
// Chromium base used by the audited Helium revision (151.0.7922.173):
// https://chromium.googlesource.com/chromium/src/+/a96602f30358e9b5d256a0464e7e4d4bec223004/
// Helium patch set:
// https://github.com/imputnet/helium/tree/208a56bab803ceda8202f8c79dd0e9bfcffe600a
const (
	crxExtensionDir        = "extensions/crx"
	zipExtensionArchiveDir = "extensions/zip"
	unpackedExtensionDir   = "extensions/unpacked"
	externalStateFile      = "extensions/external-definitions.json"
	externalStateVersion   = 1
	crxFileExtension       = ".crx"
	zipFileExtension       = ".zip"
	manifestFilename       = "manifest.json"
	manifestKeyMaxBytes    = 100 * 1024
	manifestKeyPEMBegin    = "-----BEGIN"
	manifestKeyPEMEnd      = "-----END"
	manifestKeyHeaderEnd   = "KEY-----"

	// Chromium's external-pref provider accepts exactly one of these shapes:
	// {external_crx, external_version} for a local CRX, or
	// {external_update_url} for a browser-managed download. The filename stem
	// supplies the extension ID; it is intentionally not repeated in the JSON.
	// Source: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/chrome/browser/extensions/external_provider_impl.cc#300
	externalCRXPathKey   = "external_crx"
	externalVersionKey   = "external_version"
	externalUpdateURLKey = "external_update_url"

	externalManifestFileExtension = ".json"
	chromeStoreResponseKey        = "response"
	chromeStoreProductVersionKey  = "prodversion"
	chromeStoreAcceptFormatKey    = "acceptformat"
	chromeStoreOSKey              = "os"
	chromeStoreArchitectureKey    = "arch"
	chromeStoreProductKey         = "prod"
	chromeStoreExtensionKey       = "x"
	chromeStoreResponseRedirect   = "redirect"
	// This installer needs a complete CRX archive, so it requests CRX3 only. The
	// Chromium updater also advertises "puff", but this installer has no delta
	// patch pipeline; current Chromium's verifier and our go-crx3 preflight
	// parser also do not accept CRX2.
	// Chromium updater query: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/components/update_client/update_query_params.cc#89
	// Chromium verifier formats: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/components/crx_file/crx_verifier.h#16
	chromeStoreCRXFormats    = "crx3"
	chromeStoreProduct       = "chromiumcrx"
	chromeStoreInstallSuffix = "&uc"
)

type installedCRXExtension struct {
	DownloadedExtension
	Path string
}

type externalExtensionDefinition struct {
	ID       string
	Manifest externalExtensionManifest
}

// externalExtensionManifest remains string-only because every value Chromium
// accepts in the two external-pref shapes above is a string. Keeping this type
// narrow also makes ownership-state comparison exact and fail-closed.
type externalExtensionManifest map[string]string
type externalExtensionDirectoryState map[string]externalExtensionManifest

// externalExtensionState is browser's private ownership ledger, not a file
// Chromium reads. It records the exact definitions written during the previous
// run so stale files can be removed only when the user has not edited them.
type externalExtensionState struct {
	Version     int                                        `json:"version"`
	Directories map[string]externalExtensionDirectoryState `json:"directories"`
}

type extensionVerifiers struct {
	file func(path, checksum string) error
	crx  func(path, extensionID string) error
}

type unpackedExtensionPaths struct {
	archive     string
	unpackedDir string
	target      string
	tempPrefix  string
}

type installedUnpackedExtension struct {
	ID   string
	Path string
}

func Install(ctx context.Context, options Options) (Result, error) {
	if err := ValidateCatalog(options.Catalog); err != nil {
		return Result{}, err
	}
	if options.Root == "" {
		return Result{}, errors.New("extension installation root is required")
	}
	if options.Download == nil && catalogRequiresDownload(options.Catalog) {
		return Result{}, errors.New("extension download function is required")
	}
	if err := normalizeInstallPaths(&options); err != nil {
		return Result{}, err
	}
	crxDir := filepath.Join(options.Root, crxExtensionDir)
	if err := os.MkdirAll(crxDir, fileutil.DefaultDirPerm); err != nil {
		return Result{}, err
	}
	verifiers := options.verifiers()
	installed, err := installChromeStoreExtensions(ctx, options, crxDir, verifiers.crx)
	if err != nil {
		return Result{}, err
	}
	downloaded, err := installDownloadedExtensions(ctx, options, crxDir, verifiers)
	if err != nil {
		return Result{}, err
	}
	installed = append(installed, downloaded...)
	result, err := installUnpackedExtensions(ctx, options, verifiers.file)
	if err != nil {
		return Result{}, err
	}
	definitions := externalExtensionDefinitions(options, installed)
	if err := syncExternalExtensionDefinitions(options, definitions); err != nil {
		return Result{}, err
	}
	return result, nil
}

func catalogRequiresDownload(catalog Catalog) bool {
	return len(catalog.ChromeStore) > 0 || len(catalog.CRX) > 0 || len(catalog.ZIP) > 0
}

func normalizeInstallPaths(options *Options) error {
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return fmt.Errorf("resolve extension installation root: %w", err)
	}
	options.Root = filepath.Clean(root)

	directories := make([]string, 0, len(options.ExternalDirs))
	seen := map[string]bool{}
	for index, directory := range options.ExternalDirs {
		if strings.TrimSpace(directory) == "" {
			return fmt.Errorf("external extension directory %d must not be empty", index)
		}
		absolute, err := filepath.Abs(directory)
		if err != nil {
			return fmt.Errorf("resolve external extension directory %q: %w", directory, err)
		}
		absolute = filepath.Clean(absolute)
		if seen[absolute] {
			continue
		}
		seen[absolute] = true
		directories = append(directories, absolute)
	}
	options.ExternalDirs = directories
	return nil
}

func (options Options) verifiers() extensionVerifiers {
	verifiers := extensionVerifiers{
		file: options.Verify,
		crx:  options.VerifyCRX,
	}
	if verifiers.file == nil {
		verifiers.file = VerifyFileSHA256
	}
	if verifiers.crx == nil {
		verifiers.crx = verifyCRXID
	}
	return verifiers
}

func installChromeStoreExtensions(
	ctx context.Context,
	options Options,
	crxDir string,
	verifyCRX func(path, extensionID string) error,
) ([]installedCRXExtension, error) {
	installed := make(
		[]installedCRXExtension,
		0,
		len(options.Catalog.ChromeStore),
	)
	for _, extension := range options.Catalog.ChromeStore {
		if options.ExcludedIDs[extension.ID] {
			continue
		}
		if options.ChromeVersion == "" {
			return nil, errors.New(
				"chrome product version is required for Chrome Web Store extensions",
			)
		}
		downloadURL, err := ChromeStoreCRXDownloadURLForVersion(
			options.Catalog.ChromeStoreUpdateURL,
			extension.ID,
			options.ChromeVersion,
		)
		if err != nil {
			return nil, err
		}
		resolvedURL := downloadURL
		if options.Resolve != nil {
			resolvedURL, err = options.Resolve(ctx, downloadURL)
			if err != nil {
				return nil, err
			}
		}
		version, err := ChromeStoreVersionFromCRXURL(extension.ID, resolvedURL)
		if err != nil {
			return nil, err
		}
		installedExtension, err := installCRXExtension(
			ctx,
			crxDir,
			DownloadedExtension{
				ID: extension.ID, Name: extension.Name, Version: version, URL: downloadURL,
			},
			options.Download,
			func(path string) error { return verifyCRX(path, extension.ID) },
		)
		if err != nil {
			return nil, err
		}
		installed = append(installed, installedExtension)
	}
	return installed, nil
}

func installDownloadedExtensions(
	ctx context.Context,
	options Options,
	crxDir string,
	verifiers extensionVerifiers,
) ([]installedCRXExtension, error) {
	installed := make([]installedCRXExtension, 0, len(options.Catalog.CRX))
	for _, extension := range options.Catalog.CRX {
		if options.ExcludedIDs[extension.ID] {
			continue
		}
		installedExtension, err := installCRXExtension(
			ctx,
			crxDir,
			extension,
			options.Download,
			func(path string) error {
				if err := verifiers.file(path, extension.SHA256); err != nil {
					return fmt.Errorf("verify %s: %w", extension.Name, err)
				}
				return verifiers.crx(path, extension.ID)
			},
		)
		if err != nil {
			return nil, err
		}
		installed = append(installed, installedExtension)
	}
	return installed, nil
}

func installUnpackedExtensions(
	ctx context.Context,
	options Options,
	verify func(path, checksum string) error,
) (Result, error) {
	result := Result{ExtensionIDAliases: map[string]string{}}
	for _, extension := range options.Catalog.ZIP {
		if options.ExcludedIDs[extension.ID] {
			continue
		}
		installed, err := installUnpackedExtension(ctx, options, verify, extension)
		if err != nil {
			return Result{}, err
		}
		if extension.LoadUnpacked {
			result.LoadExtensionPaths = append(result.LoadExtensionPaths, installed.Path)
			result.ExtensionIDAliases[extension.ID] = installed.ID
		}
	}
	for _, extension := range options.Catalog.Git {
		if options.ExcludedIDs[extension.ID] {
			continue
		}
		installed, err := installGitExtension(ctx, options, extension)
		if err != nil {
			return Result{}, err
		}
		if extension.LoadUnpacked {
			result.LoadExtensionPaths = append(result.LoadExtensionPaths, installed.Path)
			result.ExtensionIDAliases[extension.ID] = installed.ID
		}
	}
	return result, nil
}

func externalExtensionDefinitions(
	options Options,
	installed []installedCRXExtension,
) []externalExtensionDefinition {
	definitions := make(
		[]externalExtensionDefinition,
		0,
		len(installed)+len(options.Catalog.UpdateURL),
	)
	for _, extension := range installed {
		definitions = append(definitions, externalExtensionDefinition{
			ID: extension.ID,
			Manifest: map[string]string{
				externalCRXPathKey: extension.Path,
				externalVersionKey: extension.Version,
			},
		})
	}
	for _, extension := range options.Catalog.UpdateURL {
		if options.ExcludedIDs[extension.ID] {
			continue
		}
		definitions = append(definitions, externalExtensionDefinition{
			ID: extension.ID,
			Manifest: map[string]string{
				externalUpdateURLKey: extension.UpdateURL,
			},
		})
	}
	return definitions
}

// syncExternalExtensionDefinitions mirrors definitions to every configured
// Chromium search directory. Chromium scans non-recursive <extension-id>.json
// files in these directories. Removal is intentionally compare-before-delete:
// files not in the ledger, or whose contents no longer match it, belong to the
// user and survive reconciliation.
// Loader source: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/chrome/browser/extensions/external_pref_loader.cc#73
func syncExternalExtensionDefinitions(
	options Options,
	definitions []externalExtensionDefinition,
) error {
	statePath := filepath.Join(options.Root, externalStateFile)
	previous, err := readExternalExtensionState(statePath)
	if err != nil {
		return err
	}

	current := externalExtensionState{
		Version:     externalStateVersion,
		Directories: make(map[string]externalExtensionDirectoryState, len(options.ExternalDirs)),
	}
	for _, externalDir := range options.ExternalDirs {
		if err := os.MkdirAll(externalDir, fileutil.DefaultDirPerm); err != nil {
			return err
		}
		manifests := make(externalExtensionDirectoryState, len(definitions))
		for _, definition := range definitions {
			if err := writeExternalJSON(
				filepath.Join(externalDir, definition.ID+externalManifestFileExtension),
				definition.Manifest,
			); err != nil {
				return err
			}
			manifests[definition.ID] = maps.Clone(definition.Manifest)
		}
		current.Directories[externalDir] = manifests
	}

	for _, directory := range slices.Sorted(maps.Keys(previous.Directories)) {
		for _, id := range slices.Sorted(maps.Keys(previous.Directories[directory])) {
			if _, stillManaged := current.Directories[directory][id]; stillManaged {
				continue
			}
			path := filepath.Join(directory, id+externalManifestFileExtension)
			if err := removeExternalJSONIfUnchanged(
				path,
				previous.Directories[directory][id],
			); err != nil {
				return err
			}
		}
	}
	_, err = fileutil.WriteJSONIfChanged(
		statePath,
		current,
		fileutil.PrivateFilePerm,
	)
	return err
}

func readExternalExtensionState(path string) (externalExtensionState, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return externalExtensionState{
			Version:     externalStateVersion,
			Directories: map[string]externalExtensionDirectoryState{},
		}, nil
	}
	if err != nil {
		return externalExtensionState{}, err
	}
	var state externalExtensionState
	if err := json.Unmarshal(data, &state); err != nil {
		return externalExtensionState{}, fmt.Errorf(
			"parse managed external extension state %s: %w",
			path,
			err,
		)
	}
	if state.Version != externalStateVersion {
		return externalExtensionState{}, fmt.Errorf(
			"managed external extension state %s has unsupported version %d",
			path,
			state.Version,
		)
	}
	for directory, manifests := range state.Directories {
		if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
			return externalExtensionState{}, fmt.Errorf(
				"managed external extension state has invalid directory %q",
				directory,
			)
		}
		for id, manifest := range manifests {
			if !ValidExtensionID(id) {
				return externalExtensionState{}, fmt.Errorf(
					"managed external extension state has invalid extension ID %q",
					id,
				)
			}
			if !validExternalManifest(manifest) {
				return externalExtensionState{}, fmt.Errorf(
					"managed external extension state has invalid manifest for %q",
					id,
				)
			}
		}
	}
	if state.Directories == nil {
		state.Directories = map[string]externalExtensionDirectoryState{}
	}
	return state, nil
}

func validExternalManifest(manifest externalExtensionManifest) bool {
	if updateURL, ok := manifest[externalUpdateURLKey]; ok {
		return len(manifest) == 1 && ValidURL(updateURL)
	}
	crxPath, hasCRX := manifest[externalCRXPathKey]
	version, hasVersion := manifest[externalVersionKey]
	return len(manifest) == 2 && hasCRX && hasVersion &&
		filepath.IsAbs(crxPath) && filepath.Clean(crxPath) == crxPath &&
		ValidExternalVersion(version)
}

func removeExternalJSONIfUnchanged(path string, expected externalExtensionManifest) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var current externalExtensionManifest
	if err := json.Unmarshal(data, &current); err != nil || !maps.Equal(current, expected) {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale external extension definition %s: %w", path, err)
	}
	return nil
}

func installCRXExtension(
	ctx context.Context,
	directory string,
	extension DownloadedExtension,
	download DownloadFunc,
	verify func(path string) error,
) (installedCRXExtension, error) {
	target := filepath.Join(
		directory,
		extension.ID+"-"+extension.Version+crxFileExtension,
	)
	if err := ensureDownloaded(ctx, target, extension.URL, download, verify); err != nil {
		return installedCRXExtension{}, err
	}
	return installedCRXExtension{
		DownloadedExtension: extension,
		Path:                target,
	}, nil
}

func installUnpackedExtension(
	ctx context.Context,
	options Options,
	verify func(path, checksum string) error,
	extension ZIPExtension,
) (installedUnpackedExtension, error) {
	artifact, err := resolveReleaseArtifact(ctx, options, extension)
	if err != nil {
		return installedUnpackedExtension{}, err
	}
	if !ValidExternalVersion(artifact.Version) || !ValidURL(artifact.URL) || !ValidSHA256(artifact.SHA256) {
		return installedUnpackedExtension{}, fmt.Errorf(
			"%s release metadata is incomplete",
			extension.Name,
		)
	}
	paths, err := prepareUnpackedExtensionPaths(options.Root, extension.ID, artifact.Version)
	if err != nil {
		return installedUnpackedExtension{}, err
	}
	if err := ensureDownloaded(
		ctx,
		paths.archive,
		artifact.URL,
		options.Download,
		func(path string) error {
			if err := verify(path, artifact.SHA256); err != nil {
				return fmt.Errorf("verify %s: %w", extension.Name, err)
			}
			return nil
		},
	); err != nil {
		return installedUnpackedExtension{}, err
	}
	return installUnpackedArchive(paths, extension, artifact.Version)
}

func prepareUnpackedExtensionPaths(
	root,
	extensionID,
	version string,
) (unpackedExtensionPaths, error) {
	archiveDir := filepath.Join(root, zipExtensionArchiveDir)
	unpackedDir := filepath.Join(root, unpackedExtensionDir)
	for _, directory := range []string{archiveDir, unpackedDir} {
		if err := os.MkdirAll(directory, fileutil.DefaultDirPerm); err != nil {
			return unpackedExtensionPaths{}, err
		}
	}
	return unpackedExtensionPaths{
		archive: filepath.Join(
			archiveDir,
			extensionID+"-"+version+zipFileExtension,
		),
		unpackedDir: unpackedDir,
		target:      filepath.Join(unpackedDir, extensionID),
		tempPrefix:  "." + extensionID + "-",
	}, nil
}

func installUnpackedArchive(
	paths unpackedExtensionPaths,
	extension ZIPExtension,
	version string,
) (_ installedUnpackedExtension, err error) {
	temporaryDir, err := os.MkdirTemp(paths.unpackedDir, paths.tempPrefix)
	if err != nil {
		return installedUnpackedExtension{}, err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(temporaryDir)) }()
	if err := ExtractZipFile(paths.archive, temporaryDir); err != nil {
		return installedUnpackedExtension{}, fmt.Errorf("extract %s: %w", extension.Name, err)
	}
	sourceDir := filepath.Join(temporaryDir, filepath.FromSlash(extension.ArchiveRoot))
	return installUnpackedDirectory(sourceDir, paths.target, extension.Name, version)
}

func installGitExtension(
	ctx context.Context,
	options Options,
	extension GitExtension,
) (_ installedUnpackedExtension, err error) {
	if options.CheckoutGit == nil {
		return installedUnpackedExtension{}, errors.New(
			"git checkout function is required for Git extensions",
		)
	}
	unpackedDir := filepath.Join(options.Root, unpackedExtensionDir)
	if err := os.MkdirAll(unpackedDir, fileutil.DefaultDirPerm); err != nil {
		return installedUnpackedExtension{}, err
	}
	temporaryDir, err := os.MkdirTemp(unpackedDir, "."+extension.ID+"-git-")
	if err != nil {
		return installedUnpackedExtension{}, err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(temporaryDir)) }()
	revision, err := options.CheckoutGit(ctx, temporaryDir, extension)
	if err != nil {
		return installedUnpackedExtension{}, fmt.Errorf(
			"check out Git extension %s: %w",
			extension.Name,
			err,
		)
	}
	if !ValidGitCommit(revision.Commit) {
		return installedUnpackedExtension{}, fmt.Errorf(
			"%s Git revision metadata is incomplete",
			extension.Name,
		)
	}
	if extension.updatePolicy() == UpdatePolicyPinned &&
		!strings.EqualFold(revision.Commit, extension.Commit) {
		return installedUnpackedExtension{}, fmt.Errorf(
			"%s resolved Git commit %s, want pinned commit %s",
			extension.Name,
			revision.Commit,
			extension.Commit,
		)
	}
	return installUnpackedDirectory(
		temporaryDir,
		filepath.Join(unpackedDir, extension.ID),
		extension.Name,
		"",
	)
}

func installUnpackedDirectory(
	sourceDir,
	target,
	name,
	version string,
) (installedUnpackedExtension, error) {
	// Validate while the staged tree is still isolated. Replacing the whole tree
	// ensures files deleted by an update do not survive it.
	installedID, err := validateUnpackedManifest(sourceDir, name, version)
	if err != nil {
		return installedUnpackedExtension{}, err
	}
	if err := replaceDirectory(sourceDir, target); err != nil {
		return installedUnpackedExtension{}, err
	}
	if installedID == "" {
		resolvedPath, err := filepath.EvalSymlinks(target)
		if err != nil {
			return installedUnpackedExtension{}, fmt.Errorf(
				"resolve installed %s path: %w",
				name,
				err,
			)
		}
		installedID = UnpackedExtensionID(resolvedPath)
	}
	return installedUnpackedExtension{ID: installedID, Path: target}, nil
}

// replaceDirectory swaps a staged directory into place without first deleting
// the working installation. The old tree is parked beside the target and
// restored if the staged rename fails. All paths stay on the same filesystem,
// so the individual renames remain atomic.
func replaceDirectory(source, target string) (err error) {
	parent := filepath.Dir(target)
	backupRoot, err := os.MkdirTemp(parent, "."+filepath.Base(target)+"-previous-")
	if err != nil {
		return fmt.Errorf("reserve previous extension directory: %w", err)
	}
	defer func() { err = errors.Join(err, os.RemoveAll(backupRoot)) }()

	backup := filepath.Join(backupRoot, "tree")
	hadTarget := true
	if err := os.Rename(target, backup); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("park previous extension directory %s: %w", target, err)
		}
		hadTarget = false
	}
	if err := os.Rename(source, target); err != nil {
		installErr := fmt.Errorf("install extension directory %s: %w", target, err)
		if !hadTarget {
			return installErr
		}
		if restoreErr := os.Rename(backup, target); restoreErr != nil {
			return errors.Join(
				installErr,
				fmt.Errorf("restore previous extension directory %s: %w", target, restoreErr),
			)
		}
		return installErr
	}
	return nil
}

func resolveReleaseArtifact(
	ctx context.Context,
	options Options,
	extension ZIPExtension,
) (ReleaseArtifact, error) {
	if extension.updatePolicy() == UpdatePolicyPinned {
		return ReleaseArtifact{
			Version: extension.Version,
			URL:     extension.URL,
			SHA256:  extension.SHA256,
		}, nil
	}
	if options.ResolveLatestRelease == nil {
		return ReleaseArtifact{}, errors.New(
			"latest GitHub release resolver is required for latest ZIP extensions",
		)
	}
	artifact, err := options.ResolveLatestRelease(
		ctx,
		extension.Repository,
		extension.AssetTemplate,
	)
	if err != nil {
		return ReleaseArtifact{}, fmt.Errorf(
			"resolve latest %s release: %w",
			extension.Name,
			err,
		)
	}
	return artifact, nil
}

func ensureDownloaded(
	ctx context.Context,
	target,
	rawURL string,
	download DownloadFunc,
	verify func(path string) error,
) error {
	// A cached artifact is reusable only after the same verifier used for a new
	// download accepts it. A corrupt or stale cache is overwritten by Download;
	// callers choose whether that verifier means an exact SHA-256 pin, a CRX
	// declared-ID preflight, or both.
	if _, err := os.Stat(target); err == nil {
		if err := verify(target); err == nil {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := download(ctx, target, rawURL); err != nil {
		return err
	}
	return verify(target)
}

func validateUnpackedManifest(
	extensionDir,
	name,
	version string,
) (string, error) {
	data, err := os.ReadFile(filepath.Join(extensionDir, manifestFilename))
	if err != nil {
		return "", fmt.Errorf("read %s manifest: %w", name, err)
	}
	// "version" is the extension package version used for release matching;
	// it is unrelated to the integer "manifest_version" API contract. Do not
	// reject Manifest V2 here: Chromium owns manifest/API validation, and Helium
	// deliberately keeps Chromium's MV2 paths enabled.
	// Upstream gate: https://chromium.googlesource.com/chromium/src/+/a96602f30358e9b5d256a0464e7e4d4bec223004/extensions/browser/manifest_v2_handler.cc#137
	// Helium source: https://github.com/imputnet/helium/blob/208a56bab803ceda8202f8c79dd0e9bfcffe600a/patches/ungoogled-chromium/extensions-manifestv2.patch
	var manifest struct {
		Version string         `json:"version"`
		Key     jsontext.Value `json:"key"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", fmt.Errorf("parse %s manifest: %w", name, err)
	}
	if !ValidExternalVersion(manifest.Version) {
		return "", fmt.Errorf("%s manifest has invalid version %q", name, manifest.Version)
	}
	if version != "" && manifest.Version != version {
		return "", fmt.Errorf(
			"%s manifest version is %q, want %q",
			name,
			manifest.Version,
			version,
		)
	}
	if len(manifest.Key) == 0 {
		return "", nil
	}
	var key string
	if err := json.Unmarshal(manifest.Key, &key); err != nil {
		return "", fmt.Errorf("parse %s manifest public key: %w", name, err)
	}
	installedID, err := extensionIDFromManifestKey(key)
	if err != nil {
		return "", fmt.Errorf("parse %s manifest public key: %w", name, err)
	}
	return installedID, nil
}

// extensionIDFromManifestKey mirrors Extension::ParsePEMKeyBytes followed by
// crx_file::id_util::GenerateId. Chromium treats the decoded key as opaque
// bytes; parsing or re-marshalling it as X.509 would reject valid manifest
// inputs and could derive a different ID.
// Source: extensions/common/extension.cc ComputeExtensionID and
// Extension::ParsePEMKeyBytes in the audited Chromium revision.
func extensionIDFromManifestKey(input string) (string, error) {
	if input == "" || len(input) > manifestKeyMaxBytes {
		return "", errors.New("public key is empty or too large")
	}
	encoded := input
	if strings.HasPrefix(encoded, manifestKeyPEMBegin) {
		encoded = collapsePEMWhitespace(encoded)
		var hasHeader, hasFooter bool
		_, encoded, hasHeader = strings.Cut(encoded, manifestKeyHeaderEnd)
		encoded, _, hasFooter = strings.CutLast(encoded, manifestKeyPEMEnd)
		if !hasHeader || !hasFooter {
			return "", errors.New("public key has invalid PEM markers")
		}
		if encoded == "" {
			return "", errors.New("public key has empty PEM data")
		}
	}
	// encoding/base64 intentionally ignores CR and LF. Chromium's strict
	// Base64Decode does not, so reject any ASCII whitespace left after the PEM
	// newline-collapse step.
	if strings.ContainsAny(encoded, " \t\n\v\f\r") {
		return "", errors.New("public key base64 contains whitespace")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode public key: %w", err)
	}
	return extensionIDFromBytes(decoded), nil
}

// collapsePEMWhitespace mirrors CollapseWhitespaceASCII(value, true): leading
// and trailing whitespace is removed, whitespace runs become one space, and a
// run containing CR or LF is removed completely. This matters because a plain
// space inside PEM data remains invalid under Chromium's strict base64 decoder.
func collapsePEMWhitespace(value string) string {
	result := make([]byte, 0, len(value))
	inWhitespace := true
	alreadyTrimmed := true
	for index := range len(value) {
		character := value[index]
		if strings.ContainsRune(" \t\n\v\f\r", rune(character)) {
			if !inWhitespace {
				inWhitespace = true
				result = append(result, ' ')
			}
			if !alreadyTrimmed && (character == '\n' || character == '\r') {
				alreadyTrimmed = true
				result = result[:len(result)-1]
			}
			continue
		}
		inWhitespace = false
		alreadyTrimmed = false
		result = append(result, character)
	}
	if inWhitespace && !alreadyTrimmed {
		result = result[:len(result)-1]
	}
	return string(result)
}

// ExtractZipFile extracts regular files and directories without allowing an
// archive entry to escape destination. Backslashes are normalized before the
// slash-oriented clean so Windows spelling cannot hide ".."; Localize and
// IsLocal reject absolute/parent paths; os.Root keeps filesystem traversal
// beneath the already-open destination even in the presence of symlinks.
// Symlinks, devices, and all other non-regular entries are intentionally
// skipped instead of materialized.
// Root confinement: https://pkg.go.dev/os#Root
func ExtractZipFile(zipPath, destination string) (err error) {
	archive, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, archive.Close()) }()
	root, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	for _, entry := range archive.File {
		if err := extractZipEntry(root, entry); err != nil {
			return err
		}
	}
	return nil
}

func extractZipEntry(root *os.Root, entry *zip.File) (err error) {
	clean := path.Clean(strings.ReplaceAll(entry.Name, "\\", "/"))
	if clean == "." {
		return nil
	}
	target, err := filepath.Localize(clean)
	if err != nil || !filepath.IsLocal(target) {
		return fmt.Errorf("archive entry escapes destination: %s", entry.Name)
	}
	mode := entry.Mode()
	if mode.IsDir() {
		return root.MkdirAll(target, permOrDefault(mode, fileutil.DefaultDirPerm))
	}
	if !mode.IsRegular() {
		return nil
	}
	source, err := entry.Open()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, source.Close()) }()
	if err := root.MkdirAll(filepath.Dir(target), fileutil.DefaultDirPerm); err != nil {
		return err
	}
	destination, err := root.OpenFile(
		target,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		permOrDefault(mode, fileutil.DefaultFilePerm),
	)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	return errors.Join(copyErr, closeErr)
}

func permOrDefault(mode fs.FileMode, fallback fs.FileMode) fs.FileMode {
	if permission := mode.Perm(); permission != 0 {
		return permission
	}
	return fallback
}

// VerifyFileSHA256 streams path through SHA-256 and compares it with checksum.
// checksum may be lowercase/uppercase hex, "sha256:"-prefixed hex, or an SRI
// "sha256-" base64 digest; parseSHA256 requires exactly 32 decoded bytes.
// This authenticates pinned artifacts before extraction. It is separate from
// CRX proof verification, which Chromium performs when loading a CRX.
func VerifyFileSHA256(path, checksum string) (err error) {
	want, err := parseSHA256(checksum)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	verifier := want.Verifier()
	if _, err := io.Copy(verifier, file); err != nil {
		return err
	}
	if !verifier.Verified() {
		return fmt.Errorf("SHA-256 checksum mismatch for %s", path)
	}
	return nil
}

// UnpackedExtensionID mirrors Chromium's GenerateIdForPath on the supported
// POSIX platforms: SHA-256 of the normalized path bytes, first 16 digest bytes,
// hex nibbles mapped from 0-f to a-p. Callers resolve symlinks first because
// Chromium canonicalizes an unpacked path before deriving its stable ID.
// Source: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/components/crx_file/id_util.cc#65
func UnpackedExtensionID(extensionPath string) string {
	return extensionIDFromBytes([]byte(filepath.Clean(extensionPath)))
}

func extensionIDFromBytes(input []byte) string {
	sum := sha256.Sum256(input)
	id := make([]byte, extensionIDLength)
	for index, value := range sum[:extensionIDLength/extensionIDCharactersPerByte] {
		id[index*extensionIDCharactersPerByte] =
			extensionIDAlphabetStart + value>>extensionIDHighNibbleShift
		id[index*extensionIDCharactersPerByte+1] =
			extensionIDAlphabetStart + value&extensionIDNibbleMask
	}
	return string(id)
}

// ChromeStoreCRXDownloadURLForVersion builds the redirect form of the Omaha
// extension update request. The x value is itself an encoded query fragment
// ("id=<id>&uc"); prodversion participates in server-side compatibility
// selection, and acceptformat asks the service for a complete CRX artifact.
// This request runs through Options.Download, outside the browser. Helium's
// in-browser extension proxy therefore cannot rewrite it; configure this
// package's update URL/network path explicitly when that privacy boundary
// matters.
// Source: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/extensions/browser/webstore_installer.cc#166
// Helium proxy patch: https://github.com/imputnet/helium/blob/208a56bab803ceda8202f8c79dd0e9bfcffe600a/patches/helium/core/proxy-extension-downloads.patch
func ChromeStoreCRXDownloadURLForVersion(updateURL, id, chromeVersion string) (string, error) {
	if !ValidExternalVersion(chromeVersion) {
		return "", fmt.Errorf("invalid Chrome product version %q", chromeVersion)
	}
	parsed, err := url.Parse(updateURL)
	if err != nil {
		return "", fmt.Errorf("parse Chrome Store update URL for %s: %w", id, err)
	}
	parsed.RawQuery = url.Values{
		chromeStoreResponseKey:       {chromeStoreResponseRedirect},
		chromeStoreProductVersionKey: {chromeVersion},
		chromeStoreAcceptFormatKey:   {chromeStoreCRXFormats},
		chromeStoreOSKey:             {chromiumUpdateOS()},
		chromeStoreArchitectureKey:   {chromiumUpdateArchitecture()},
		chromeStoreProductKey:        {chromeStoreProduct},
		chromeStoreExtensionKey:      {"id=" + id + chromeStoreInstallSuffix},
	}.Encode()
	return parsed.String(), nil
}

func chromiumUpdateOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "mac"
	case "windows":
		return "win"
	default:
		return runtime.GOOS
	}
}

func chromiumUpdateArchitecture() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "386":
		return "x86"
	case "mips64le":
		return "mips64el"
	case "mipsle":
		return "mipsel"
	case "loong64":
		return "loongarch64"
	case "ppc64le":
		return "ppc64"
	default:
		return runtime.GOARCH
	}
}

// ChromeStoreVersionFromCRXURL extracts the version from the redirect target's
// conventional <UPPERCASE_ID>_<underscore-version>.crx filename. This is update
// service response parsing, not CRX parsing; a service with another redirect
// convention must provide compatible metadata or use a pinned CRX entry.
func ChromeStoreVersionFromCRXURL(id, crxURL string) (string, error) {
	parsed, err := url.Parse(crxURL)
	if err != nil {
		return "", fmt.Errorf("parse Chrome Store CRX URL for %s: %w", id, err)
	}
	filename := filepath.Base(parsed.Path)
	prefix := strings.ToUpper(id) + "_"
	version, ok := strings.CutPrefix(filename, prefix)
	if !ok {
		return "", fmt.Errorf("parse Chrome Store CRX version for %s from %s", id, crxURL)
	}
	version, ok = strings.CutSuffix(version, crxFileExtension)
	if !ok {
		return "", fmt.Errorf("parse Chrome Store CRX version for %s from %s", id, crxURL)
	}
	version = strings.ReplaceAll(version, "_", ".")
	if !ValidExternalVersion(version) {
		return "", fmt.Errorf("invalid Chrome Store CRX version %q for %s", version, id)
	}
	return version, nil
}

// verifyCRXID checks the extension ID declared by a CRX3 container.
//
// CRX3 starts with "Cr24", little-endian version 3, and a little-endian
// protobuf-header length, followed by CrxFileHeader and the ZIP payload. The
// header's SignedData carries the 16-byte CRX ID. A real verifier must also
// prove that a developer key whose SHA-256 prefix yields that ID signed
// "CRX3 SignedData\x00" + signed-data length + signed data + ZIP payload.
//
// go-crx3.ID only reads the declared SignedData ID; it does not verify those
// proofs. Pinned CRXs are independently authenticated by their configured
// SHA-256, and Chromium verifies CRX3 proofs when it installs the external CRX.
// Keep that trust boundary explicit: this helper alone is not signature
// verification. The dependency also uses unchecked slices for hostile header
// lengths, so recover converts any parser panic into a closed verification
// failure rather than letting malformed network input terminate the process.
// Format: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/components/crx_file/crx3.proto#9
// Chromium verifier: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/components/crx_file/crx_verifier.cc#155
// Parser used here: https://github.com/mediabuyerbot/go-crx3/blob/v1.7.0/id.go#L22-L50
func verifyCRXID(path, want string) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("read CRX extension ID from %s: %v", path, recovered)
		}
	}()
	got, err := crx3.ID(path)
	if err != nil {
		return fmt.Errorf("read CRX extension ID from %s: %w", path, err)
	}
	if got != want {
		return fmt.Errorf("CRX %s has extension ID %s, want %s", path, got, want)
	}
	return nil
}

func writeExternalJSON(path string, value map[string]string) error {
	_, err := fileutil.WriteJSONIfChanged(path, value, fileutil.DefaultFilePerm)
	return err
}
