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
	externalVersionPartBits      = 16
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

func ValidateCatalog(catalog Catalog) error {
	var errs []error
	errs = append(errs, validateNetworkConfig(catalog.Network))
	if len(catalog.ChromeStore) > 0 && !ValidURL(catalog.ChromeStoreUpdateURL) {
		errs = append(
			errs,
			errors.New("extensions.chrome_store_update_url is required when Chrome Store entries exist"),
		)
	}
	for _, extension := range catalog.ChromeStore {
		errs = append(errs, validateExtensionFields(
			"chrome store",
			extension.Name,
			extensionFieldValidation{name: "ID", valid: ValidExtensionID(extension.ID)},
		))
	}
	for _, extension := range catalog.UpdateURL {
		errs = append(errs, validateExtensionFields(
			"update URL",
			extension.Name,
			extensionFieldValidation{name: "ID", valid: ValidExtensionID(extension.ID)},
			extensionFieldValidation{name: "update URL", valid: ValidURL(extension.UpdateURL)},
		))
	}
	for _, extension := range catalog.CRX {
		errs = append(errs, validateExtensionFields(
			"downloaded",
			extension.Name,
			extensionFieldValidation{name: "ID", valid: ValidExtensionID(extension.ID)},
			extensionFieldValidation{
				name: "version", valid: ValidExternalVersion(extension.Version),
			},
			extensionFieldValidation{name: "URL", valid: ValidURL(extension.URL)},
			extensionFieldValidation{name: "SHA-256", valid: ValidSHA256(extension.SHA256)},
		))
	}
	for _, extension := range catalog.ZIP {
		errs = append(errs, validateGitHubReleaseExtension(extension))
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

func validateGitHubReleaseExtension(extension GitHubReleaseExtension) error {
	errs := []error{validateExtensionFields(
		"ZIP",
		extension.Name,
		extensionFieldValidation{name: "ID", valid: ValidExtensionID(extension.ID)},
	)}
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

func validateLatestReleaseExtension(extension GitHubReleaseExtension) error {
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

func validatePinnedReleaseExtension(extension GitHubReleaseExtension) error {
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

func (extension GitHubReleaseExtension) updatePolicy() UpdatePolicy {
	if extension.UpdatePolicy != "" {
		return extension.UpdatePolicy
	}
	if extension.Version != "" || extension.URL != "" || extension.SHA256 != "" {
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

func ValidExternalVersion(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) == 0 || len(parts) > maxExternalVersionParts {
		return false
	}
	nonzero := false
	for _, part := range parts {
		if part == "" || len(part) > 1 && part[0] == '0' {
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
		nonzero = nonzero || value != 0
	}
	return nonzero
}

func ValidGitHubRepository(repository string) bool {
	owner, name, ok := strings.Cut(repository, "/")
	return ok && owner != "" && name != "" && !strings.Contains(name, "/")
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
		checksum = strings.TrimPrefix(checksum, sha256HexPrefix)
		raw, err = hex.DecodeString(checksum)
	}
	if err != nil || len(raw) != digest.SHA256.Size() {
		return "", fmt.Errorf("invalid SHA-256 checksum %q", original)
	}
	return digest.NewDigestFromBytes(digest.SHA256, raw), nil
}
