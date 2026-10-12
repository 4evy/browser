package chromium

import (
	"context"
	"io"

	extensionstorage "github.com/4evy/tweezle/addons/foundation/chrome/storage"
	jsonutil "github.com/4evy/tweezle/internal/json"
)

type (
	ExtensionStorageArea          = extensionstorage.Area
	ExtensionStorageEncoding      = extensionstorage.Encoding
	ExtensionStorageOperationKind = extensionstorage.OperationKind
	SettingsSource                = extensionstorage.SettingsSource
	ExtensionStorageSettings      = extensionstorage.Settings
	ExtensionStorageEntry         = extensionstorage.Entry
	ExtensionStorageOperation     = extensionstorage.Operation
	ExtensionStorageInput         = extensionstorage.InputBinding
)

type ApplyOptions struct {
	liteManifestPath   string
	BrowserExecutable  string
	ExtensionRoot      string
	ProfileDir         string
	Settings           []string
	SettingsSource     []SettingsSource
	ExtensionIDAliases map[string]string
	Input              ApplyInput
}

type ApplyInput struct {
	CookieAllowlist []string       `json:"cookie_allowlist"`
	ExtensionValues map[string]any `json:"extension_values"`
}

const (
	ExtensionStorageAreaLocal = extensionstorage.AreaLocal
	ExtensionStorageAreaSync  = extensionstorage.AreaSync

	ExtensionStorageEncodingJSON        = extensionstorage.EncodingJSON
	ExtensionStorageEncodingLZStringURI = extensionstorage.EncodingLZStringURI

	ExtensionStorageOperationSet    = extensionstorage.OperationSet
	ExtensionStorageOperationMerge  = extensionstorage.OperationMerge
	ExtensionStorageOperationAppend = extensionstorage.OperationAppend
	ExtensionStorageOperationRemove = extensionstorage.OperationRemove
	ExtensionStorageOperationClear  = extensionstorage.OperationClear
)

func ApplyExtensionSettings(ctx context.Context, options ApplyOptions) error {
	if err := EnsureProfileNotRunning(options.ProfileDir); err != nil {
		return err
	}
	return extensionstorage.Apply(ctx, extensionstorage.ApplyOptions{
		ProfileDir:         options.ProfileDir,
		Settings:           options.Settings,
		SettingsSource:     options.SettingsSource,
		ExtensionIDAliases: options.ExtensionIDAliases,
		Input: extensionstorage.Input{
			ExtensionValues: options.Input.ExtensionValues,
		},
	})
}

func ValidateExtensionSettingsFiles(paths []string) error {
	return extensionstorage.ValidateFiles(paths)
}

// DecodeApplyInput reads exactly one input document and rejects unknown fields
func DecodeApplyInput(reader io.Reader) (ApplyInput, error) {
	return jsonutil.Strict.Decode[ApplyInput](reader)
}

func isStorageTemporarilyUnavailable(err error) bool {
	return extensionstorage.IsTemporarilyUnavailable(err)
}
