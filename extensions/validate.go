package extensions

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/opencontainers/go-digest"
	"golang.org/x/net/http/httpguts"
)

const (
	releaseTagPlaceholder        = "{tag}"
	releaseVersionPrefix         = "v"
	sha256SRIprefix              = "sha256-"
	sha256HexPrefix              = "sha256:"
	extensionIDLength            = 32
	extensionIDCharactersPerByte = 2
	extensionIDHighNibbleShift   = 4
	extensionIDNibbleMask        = 1<<extensionIDHighNibbleShift - 1
	extensionIDAlphabetStart     = 'a'
	extensionIDAlphabetEnd       = 'p'
	externalVersionRadix         = 10
	externalVersionPartBits      = 32
	maxExternalVersionParts      = 4
)

type extensionFieldValidation struct {
	name  string
	valid bool
}

type nonNegativeNetworkSetting struct {
	name  string
	value int
}

type catalogExtensionID struct {
	id   string
	kind string
	name string
}

type identifiedCatalogExtension interface {
	catalogIdentity() (string, string)
}

type catalogValidator struct {
	errs    []error
	entries []catalogExtensionID
}

// Add validates one homogeneous catalog section and records its identities
// for the cross-section duplicate check. Method-local type parameters let the
// section declaration remain typed without repeating an iteration per phase.
func (validator *catalogValidator) Add[T identifiedCatalogExtension](
	kind string,
	extensions []T,
	validate func(T) error,
) {
	for _, extension := range extensions {
		id, name := extension.catalogIdentity()
		validator.entries = append(validator.entries, catalogExtensionID{
			id: id, kind: kind, name: name,
		})
		validator.errs = append(validator.errs, validateExtensionFields(
			kind,
			name,
			extensionFieldValidation{name: "ID", valid: ValidExtensionID(id)},
		))
		if validate != nil {
			validator.errs = append(validator.errs, validate(extension))
		}
	}
}

func ValidateCatalog(catalog Catalog) error {
	validator := catalogValidator{}
	validator.errs = append(validator.errs, validateNetworkConfig(catalog.Network))
	if len(catalog.ChromeStore) > 0 && !ValidURL(catalog.ChromeStoreUpdateURL) {
		validator.errs = append(
			validator.errs,
			errors.New("extensions.chrome_store_update_url is required when Chrome Store entries exist"),
		)
	}
	validator.Add("chrome store", catalog.ChromeStore, nil)
	validator.Add("update URL", catalog.UpdateURL, validateUpdateURLExtension)
	validator.Add("downloaded", catalog.CRX, validateDownloadedExtension)
	validator.Add("ZIP", catalog.ZIP, validateZIPExtension)
	validator.Add("Git", catalog.Git, validateGitExtension)
	validator.errs = append(
		validator.errs,
		validateUniqueExtensionIDs(validator.entries),
	)
	return errors.Join(validator.errs...)
}

func validateUniqueExtensionIDs(entries []catalogExtensionID) error {
	var errs []error
	seen := map[string]catalogExtensionID{}
	for _, entry := range entries {
		if !ValidExtensionID(entry.id) {
			continue
		}
		if previous, exists := seen[entry.id]; exists {
			errs = append(errs, fmt.Errorf(
				"extension ID %q is declared more than once (%s %q and %s %q)",
				entry.id,
				previous.kind,
				previous.name,
				entry.kind,
				entry.name,
			))
			continue
		}
		seen[entry.id] = entry
	}
	return errors.Join(errs...)
}

func validateUpdateURLExtension(extension UpdateURLExtension) error {
	return validateExtensionFields(
		"update URL",
		extension.Name,
		extensionFieldValidation{name: "update URL", valid: ValidURL(extension.UpdateURL)},
	)
}

func validateDownloadedExtension(extension DownloadedExtension) error {
	return validateExtensionFields(
		"downloaded",
		extension.Name,
		extensionFieldValidation{
			name: "version", valid: ValidExternalVersion(extension.Version),
		},
		extensionFieldValidation{name: "URL", valid: ValidURL(extension.URL)},
		extensionFieldValidation{name: "SHA-256", valid: ValidSHA256(extension.SHA256)},
	)
}

// ValidateIDAliases verifies source and installed Chromium extension IDs.
func ValidateIDAliases(aliases map[string]string) error {
	var errs []error
	for _, sourceID := range slices.Sorted(maps.Keys(aliases)) {
		installedID := aliases[sourceID]
		if !ValidExtensionID(sourceID) {
			errs = append(errs, fmt.Errorf("invalid extension ID alias source %q", sourceID))
		}
		if !ValidExtensionID(installedID) {
			errs = append(errs, fmt.Errorf(
				"invalid installed extension ID %q for alias %q",
				installedID,
				sourceID,
			))
		}
	}
	return errors.Join(errs...)
}

func validateExtensionFields(
	kind,
	name string,
	fields ...extensionFieldValidation,
) error {
	var errs []error
	for _, field := range fields {
		if !field.valid {
			errs = append(errs, fmt.Errorf(
				"%s extension %q has an invalid %s",
				kind,
				name,
				field.name,
			))
		}
	}
	return errors.Join(errs...)
}

func validateZIPExtension(extension ZIPExtension) error {
	var errs []error
	switch extension.updatePolicy() {
	case UpdatePolicyLatest:
		errs = append(errs, validateLatestReleaseExtension(extension))
	case UpdatePolicyPinned:
		errs = append(errs, validatePinnedReleaseExtension(extension))
	default:
		errs = append(errs, fmt.Errorf(
			"ZIP extension %q update_policy must be latest or pinned",
			extension.Name,
		))
	}
	if extension.ArchiveRoot != "" &&
		!filepath.IsLocal(filepath.FromSlash(extension.ArchiveRoot)) {
		errs = append(errs, fmt.Errorf(
			"ZIP extension %q has an invalid archive root",
			extension.Name,
		))
	}
	return errors.Join(errs...)
}

func validateLatestReleaseExtension(extension ZIPExtension) error {
	var errs []error
	if extension.Version != "" || extension.URL != "" || extension.SHA256 != "" {
		errs = append(errs, fmt.Errorf(
			"latest ZIP extension %q must not set version, URL, or SHA-256 pin fields",
			extension.Name,
		))
	}
	if !ValidGitHubRepository(extension.Repository) {
		errs = append(errs, fmt.Errorf(
			"latest ZIP extension %q has an invalid GitHub repository",
			extension.Name,
		))
	}
	if !strings.Contains(extension.AssetTemplate, releaseTagPlaceholder) {
		errs = append(errs, fmt.Errorf(
			"latest ZIP extension %q asset template must contain %q",
			extension.Name,
			releaseTagPlaceholder,
		))
	}
	return errors.Join(errs...)
}

func validatePinnedReleaseExtension(extension ZIPExtension) error {
	var errs []error
	if extension.Repository != "" || extension.AssetTemplate != "" {
		errs = append(errs, fmt.Errorf(
			"pinned ZIP extension %q must not set latest-release repository or asset_template fields",
			extension.Name,
		))
	}
	errs = append(errs, validateExtensionFields(
		"pinned ZIP",
		extension.Name,
		extensionFieldValidation{
			name: "version", valid: ValidExternalVersion(extension.Version),
		},
		extensionFieldValidation{name: "URL", valid: ValidURL(extension.URL)},
		extensionFieldValidation{name: "SHA-256", valid: ValidSHA256(extension.SHA256)},
	))
	return errors.Join(errs...)
}

func (extension ZIPExtension) updatePolicy() UpdatePolicy {
	if extension.UpdatePolicy != "" {
		return extension.UpdatePolicy
	}
	if extension.Version != "" || extension.URL != "" || extension.SHA256 != "" {
		return UpdatePolicyPinned
	}
	return UpdatePolicyLatest
}

func validateGitExtension(extension GitExtension) error {
	var errs []error
	provider := extension.gitProvider()
	switch provider {
	case GitProviderGit:
		if extension.Repository != "" {
			errs = append(errs, fmt.Errorf(
				"git extension %q with provider %q must not set repository",
				extension.Name,
				provider,
			))
		}
		if !ValidGitURL(extension.URL) {
			errs = append(errs, fmt.Errorf(
				"git extension %q has an invalid Git URL",
				extension.Name,
			))
		}
	case GitProviderGitHub, GitProviderGitLab, GitProviderCodeberg, GitProviderSourceHut:
		if extension.URL != "" {
			errs = append(errs, fmt.Errorf(
				"git extension %q with provider %q must not set URL",
				extension.Name,
				provider,
			))
		}
		if !ValidHostedGitRepository(provider, extension.Repository) {
			errs = append(errs, fmt.Errorf(
				"git extension %q has an invalid %s repository",
				extension.Name,
				provider,
			))
		}
	default:
		errs = append(errs, fmt.Errorf(
			"git extension %q has unsupported provider %q",
			extension.Name,
			provider,
		))
	}
	if extension.Ref != "" && !ValidGitRef(extension.Ref) {
		errs = append(errs, fmt.Errorf(
			"git extension %q has an invalid ref",
			extension.Name,
		))
	}
	switch extension.updatePolicy() {
	case UpdatePolicyLatest:
		if extension.Commit != "" {
			errs = append(errs, fmt.Errorf(
				"latest Git extension %q must not set commit",
				extension.Name,
			))
		}
	case UpdatePolicyPinned:
		if !ValidGitCommit(extension.Commit) {
			errs = append(errs, fmt.Errorf(
				"pinned Git extension %q has an invalid commit",
				extension.Name,
			))
		}
	default:
		errs = append(errs, fmt.Errorf(
			"git extension %q update_policy must be latest or pinned",
			extension.Name,
		))
	}
	if extension.Subdirectory != "" &&
		(!filepath.IsLocal(filepath.FromSlash(extension.Subdirectory)) ||
			strings.ContainsAny(extension.Subdirectory, ":\\")) {
		errs = append(errs, fmt.Errorf(
			"git extension %q has an invalid subdirectory",
			extension.Name,
		))
	}
	return errors.Join(errs...)
}

func (extension GitExtension) gitProvider() GitProvider {
	if extension.Provider == "" && extension.URL != "" {
		return GitProviderGit
	}
	return extension.Provider
}

func (extension GitExtension) updatePolicy() UpdatePolicy {
	if extension.UpdatePolicy != "" {
		return extension.UpdatePolicy
	}
	if extension.Commit != "" {
		return UpdatePolicyPinned
	}
	return UpdatePolicyLatest
}

func validateNetworkConfig(config NetworkConfig) error {
	var errs []error
	if config.ChromeVersion != "" && !ValidExternalVersion(config.ChromeVersion) {
		errs = append(errs, fmt.Errorf(
			"extensions.network.chrome_version is invalid: %q",
			config.ChromeVersion,
		))
	}
	if !httpguts.ValidHeaderFieldValue(config.UserAgent) {
		errs = append(errs, errors.New(
			"extensions.network.user_agent contains an invalid control character",
		))
	}
	retryMax := 0
	if config.RetryMax != nil {
		retryMax = *config.RetryMax
	}
	errs = append(errs, validateNonNegativeNetworkSettings(
		nonNegativeNetworkSetting{
			name: "timeout_seconds", value: config.TimeoutSeconds,
		},
		nonNegativeNetworkSetting{
			name: "retry_max", value: retryMax,
		},
		nonNegativeNetworkSetting{
			name:  "retry_wait_min_milliseconds",
			value: config.RetryWaitMinMilliseconds,
		},
		nonNegativeNetworkSetting{
			name:  "retry_wait_max_milliseconds",
			value: config.RetryWaitMaxMilliseconds,
		},
	))
	if config.RetryWaitMaxMilliseconds > 0 &&
		config.RetryWaitMinMilliseconds > config.RetryWaitMaxMilliseconds {
		errs = append(errs, errors.New(
			"extensions.network.retry_wait_min_milliseconds must not exceed retry_wait_max_milliseconds",
		))
	}
	errs = append(errs, validateNetworkHeaders(config.Headers))
	return errors.Join(errs...)
}

func validateNonNegativeNetworkSettings(settings ...nonNegativeNetworkSetting) error {
	var errs []error
	for _, setting := range settings {
		if setting.value < 0 {
			errs = append(errs, fmt.Errorf(
				"extensions.network.%s must not be negative",
				setting.name,
			))
		}
	}
	return errors.Join(errs...)
}

func validateNetworkHeaders(headers map[string]string) error {
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(headers)) {
		value := headers[name]
		if !httpguts.ValidHeaderFieldName(name) {
			errs = append(errs, fmt.Errorf("extensions.network header name %q is invalid", name))
		}
		if !httpguts.ValidHeaderFieldValue(value) {
			errs = append(errs, fmt.Errorf("extensions.network header %q has an invalid value", name))
		}
	}
	return errors.Join(errs...)
}

// ValidExtensionID accepts Chromium's canonical extension-ID spelling: exactly
// 32 lowercase characters in a-p. Chromium's IdIsValid also accepts uppercase
// input by folding ASCII case, but persisted IDs and generated IDs are
// lowercase; requiring that canonical form prevents case-mismatched filenames
// and aliases.
// Source: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/components/crx_file/id_util.cc#35
func ValidExtensionID(id string) bool {
	if len(id) != extensionIDLength {
		return false
	}
	for _, character := range id {
		if character < extensionIDAlphabetStart || character > extensionIDAlphabetEnd {
			return false
		}
	}
	return true
}

func ValidURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

// ValidExternalVersion mirrors the version accepted by Chromium when it loads
// an extension manifest: one to four uint32 decimal components. base::Version
// requires only the first component to use canonical spelling; later
// components may contain leading zeroes, which Chromium accepts with an
// install warning. Zero versions are valid too.
// Parser: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/base/version.cc#21
// Extension limit: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/extensions/common/extension.cc#696
func ValidExternalVersion(version string) bool {
	partCount := 0
	for part := range strings.SplitSeq(version, ".") {
		partCount++
		if partCount > maxExternalVersionParts {
			return false
		}
		if part == "" || part[0] == '+' {
			return false
		}
		value, err := strconv.ParseUint(
			part,
			externalVersionRadix,
			externalVersionPartBits,
		)
		if err != nil {
			return false
		}
		if partCount == 1 && strconv.FormatUint(value, externalVersionRadix) != part {
			return false
		}
	}
	return partCount > 0
}

func ValidGitHubRepository(repository string) bool {
	owner, name, ok := strings.Cut(repository, "/")
	return ok && owner != "" && name != "" && !strings.Contains(name, "/")
}

func ValidHostedGitRepository(provider GitProvider, repository string) bool {
	if repository == "" || repository != strings.Trim(repository, "/") ||
		strings.HasSuffix(repository, ".git") ||
		strings.ContainsAny(repository, "\\:#?\t\r\n ") {
		return false
	}
	parts := strings.Split(repository, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	switch provider {
	case GitProviderGitHub, GitProviderCodeberg:
		return len(parts) == 2 && !strings.HasPrefix(parts[0], "~")
	case GitProviderGitLab:
		return len(parts) >= 2 && !strings.HasPrefix(parts[0], "~")
	case GitProviderSourceHut:
		return len(parts) == 2 && len(parts[0]) > 1 && strings.HasPrefix(parts[0], "~")
	default:
		return false
	}
}

func ValidGitURL(rawURL string) bool {
	if rawURL == "" || rawURL != strings.TrimSpace(rawURL) ||
		strings.ContainsAny(rawURL, "\x00\r\n\t") || strings.HasPrefix(rawURL, "-") {
		return false
	}
	if before, after, ok := strings.Cut(rawURL, ":"); ok &&
		!strings.Contains(before, "/") && strings.Contains(before, "@") {
		return after != "" && !strings.HasPrefix(after, "/")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	switch parsed.Scheme {
	case "http", "https", "ssh", "git":
		return parsed.Host != "" && parsed.Path != ""
	case "file":
		return parsed.Host == "" && filepath.IsAbs(parsed.Path)
	default:
		return false
	}
}

func ValidGitRef(ref string) bool {
	if ref == "" || strings.HasPrefix(ref, "-") || strings.HasPrefix(ref, "/") ||
		strings.HasSuffix(ref, "/") || strings.HasSuffix(ref, ".") ||
		strings.Contains(ref, "..") || strings.Contains(ref, "@{") ||
		strings.Contains(ref, "//") || strings.ContainsAny(ref, " ~^:?*[\\\x7f") {
		return false
	}
	for _, character := range ref {
		if character < 0x20 {
			return false
		}
	}
	for part := range strings.SplitSeq(ref, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return true
}

func ValidGitCommit(commit string) bool {
	if len(commit) != 40 && len(commit) != 64 {
		return false
	}
	_, err := hex.DecodeString(commit)
	return err == nil
}

func ValidSHA256(checksum string) bool {
	_, err := parseSHA256(checksum)
	return err == nil
}

func NormalizeSHA256(checksum string) (string, error) {
	value, err := parseSHA256(checksum)
	if err != nil {
		return "", err
	}
	raw, err := hex.DecodeString(value.Encoded())
	if err != nil {
		return "", err
	}
	return sha256SRIprefix + base64.StdEncoding.EncodeToString(raw), nil
}

func parseSHA256(checksum string) (digest.Digest, error) {
	original := checksum
	var (
		raw []byte
		err error
	)
	if encoded, ok := strings.CutPrefix(checksum, sha256SRIprefix); ok {
		raw, err = base64.StdEncoding.DecodeString(encoded)
	} else {
		checksum, _ = strings.CutPrefix(checksum, sha256HexPrefix)
		raw, err = hex.DecodeString(checksum)
	}
	if err != nil || len(raw) != digest.SHA256.Size() {
		return "", fmt.Errorf("invalid SHA-256 checksum %q", original)
	}
	return digest.NewDigestFromBytes(digest.SHA256, raw), nil
}
