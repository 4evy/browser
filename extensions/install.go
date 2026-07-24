package extensions

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/4evy/browser/internal/fileutil"
	crx3 "github.com/mediabuyerbot/go-crx3"
)

const (
	crxExtensionDir        = "extensions/crx"
	zipExtensionArchiveDir = "extensions/zip"
	unpackedExtensionDir   = "extensions/unpacked"
	crxFileExtension       = ".crx"
	zipFileExtension       = ".zip"
	manifestFilename       = "manifest.json"

	externalCRXPathKey   = "external_crx"
	externalVersionKey   = "external_version"
	externalUpdateURLKey = "external_update_url"

	externalManifestFileExtension = ".json"
	chromeStoreResponseKey        = "response"
	chromeStoreProductVersionKey  = "prodversion"
	chromeStoreAcceptFormatKey    = "acceptformat"
	chromeStoreExtensionKey       = "x"
	chromeStoreResponseRedirect   = "redirect"
	chromeStoreCRXFormats         = "crx2,crx3"
	chromeStoreInstallSuffix      = "&uc"
)

type installedCRXExtension struct {
	DownloadedExtension
	Path string
}

type externalExtensionDefinition struct {
	ID       string
	Manifest map[string]string
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

func Install(ctx context.Context, options Options) (Result, error) {
	if err := ValidateCatalog(options.Catalog); err != nil {
		return Result{}, err
	}
	if options.Root == "" {
		return Result{}, errors.New("extension installation root is required")
	}
	if options.Download == nil {
		return Result{}, errors.New("extension download function is required")
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
	if err := writeExternalExtensionDefinitions(options.ExternalDirs, definitions); err != nil {
		return Result{}, err
	}
	return result, nil
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
		extensionPath, err := installUnpackedExtension(ctx, options, verify, extension)
		if err != nil {
			return Result{}, err
		}
		if extension.LoadUnpacked {
			result.LoadExtensionPaths = append(result.LoadExtensionPaths, extensionPath)
			result.ExtensionIDAliases[extension.ID] = UnpackedExtensionID(extensionPath)
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

func writeExternalExtensionDefinitions(
	directories []string,
	definitions []externalExtensionDefinition,
) error {
	for _, externalDir := range directories {
		if err := os.MkdirAll(externalDir, fileutil.DefaultDirPerm); err != nil {
			return err
		}
		for _, definition := range definitions {
			if err := writeExternalJSON(
				filepath.Join(externalDir, definition.ID+externalManifestFileExtension),
				definition.Manifest,
			); err != nil {
				return err
			}
		}
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
	extension GitHubReleaseExtension,
) (_ string, err error) {
	artifact, err := resolveReleaseArtifact(ctx, options, extension)
	if err != nil {
		return "", err
	}
	if !ValidExternalVersion(artifact.Version) || !ValidURL(artifact.URL) || !ValidSHA256(artifact.SHA256) {
		return "", fmt.Errorf("%s release metadata is incomplete", extension.Name)
	}
	paths, err := prepareUnpackedExtensionPaths(options.Root, extension.ID, artifact.Version)
	if err != nil {
		return "", err
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
		return "", err
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
	extension GitHubReleaseExtension,
	version string,
) (_ string, err error) {
	temporaryDir, err := os.MkdirTemp(paths.unpackedDir, paths.tempPrefix)
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(temporaryDir)) }()
	if err := ExtractZipFile(paths.archive, temporaryDir); err != nil {
		return "", fmt.Errorf("extract %s: %w", extension.Name, err)
	}
	sourceDir := filepath.Join(temporaryDir, filepath.FromSlash(extension.ArchiveRoot))
	if err := validateUnpackedManifest(sourceDir, extension.Name, version); err != nil {
		return "", err
	}
	if err := os.RemoveAll(paths.target); err != nil {
		return "", err
	}
	if err := os.Rename(sourceDir, paths.target); err != nil {
		return "", err
	}
	return paths.target, nil
}

func resolveReleaseArtifact(
	ctx context.Context,
	options Options,
	extension GitHubReleaseExtension,
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

func validateUnpackedManifest(extensionDir, name, version string) error {
	data, err := os.ReadFile(filepath.Join(extensionDir, manifestFilename))
	if err != nil {
		return fmt.Errorf("read %s manifest: %w", name, err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parse %s manifest: %w", name, err)
	}
	if manifest.Version != version {
		return fmt.Errorf("%s manifest version is %q, want %q", name, manifest.Version, version)
	}
	return nil
}

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

func UnpackedExtensionID(extensionPath string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(extensionPath)))
	id := make([]byte, extensionIDLength)
	for index, value := range sum[:extensionIDLength/extensionIDCharactersPerByte] {
		id[index*extensionIDCharactersPerByte] =
			extensionIDAlphabetStart + value>>extensionIDHighNibbleShift
		id[index*extensionIDCharactersPerByte+1] =
			extensionIDAlphabetStart + value&extensionIDNibbleMask
	}
	return string(id)
}

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
		chromeStoreExtensionKey:      {"id=" + id + chromeStoreInstallSuffix},
	}.Encode()
	return parsed.String(), nil
}

func ChromeStoreVersionFromCRXURL(id, crxURL string) (string, error) {
	parsed, err := url.Parse(crxURL)
	if err != nil {
		return "", fmt.Errorf("parse Chrome Store CRX URL for %s: %w", id, err)
	}
	filename := filepath.Base(parsed.Path)
	prefix := strings.ToUpper(id) + "_"
	if !strings.HasPrefix(filename, prefix) || !strings.HasSuffix(filename, crxFileExtension) {
		return "", fmt.Errorf("parse Chrome Store CRX version for %s from %s", id, crxURL)
	}
	version := strings.TrimSuffix(strings.TrimPrefix(filename, prefix), crxFileExtension)
	version = strings.ReplaceAll(version, "_", ".")
	if !ValidExternalVersion(version) {
		return "", fmt.Errorf("invalid Chrome Store CRX version %q for %s", version, id)
	}
	return version, nil
}

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
