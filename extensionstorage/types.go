// Package extensionstorage applies declarative mutations to persistent
// Chromium extension storage.
package extensionstorage

import "encoding/json/jsontext"

const (
	localExtensionSettingsDir = "Local Extension Settings"
	syncExtensionSettingsDir  = "Sync Extension Settings"

	extensionSettingsSchemaVersion = 1
	syncStorageMaxItems            = 512
	syncStorageQuotaBytes          = 100 * 1024
	syncStorageQuotaBytesPerItem   = 8 * 1024

	extensionStorageAreaChoices      = "local or sync"
	extensionStorageEncodingChoices  = "json or json-lz-string-uri"
	extensionStorageOperationChoices = "set, merge, append, remove, or clear"
	extensionStorageInputOpChoices   = "set, merge, or append"
)

// Area selects a persistent chrome.storage area. Only the disk-backed local
// and sync areas are supported; managed storage is policy-owned and session
// storage exists only in browser memory.
// Source: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/extensions/browser/api/storage/value_store_util.cc#17
type Area string

const (
	AreaLocal Area = "local"
	AreaSync  Area = "sync"
)

// Encoding describes the bytes stored as one LevelDB value. EncodingJSON is
// Chromium's native ValueStore representation: a chrome.storage key maps to
// one JSON-serialized value. EncodingLZStringURI is an extension-owned
// convention: Chromium sees a JSON string while the extension decompresses
// that string into another JSON document.
// Native JSON source: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/components/value_store/leveldb_value_store.cc#268
type Encoding string

const (
	EncodingJSON        Encoding = "json"
	EncodingLZStringURI Encoding = "json-lz-string-uri"
)

// OperationKind identifies a mutation applied to a storage value.
type OperationKind string

const (
	OperationSet    OperationKind = "set"
	OperationMerge  OperationKind = "merge"
	OperationAppend OperationKind = "append"
	OperationRemove OperationKind = "remove"
	OperationClear  OperationKind = "clear"
)

type extensionStorageOperationSpec struct {
	allowedInput bool
	scope        extensionStorageOperationScope
	value        extensionStorageValueRequirement
}

var extensionStorageOperationSpecs = map[OperationKind]extensionStorageOperationSpec{
	OperationSet: {
		allowedInput: true,
		scope:        extensionStorageOperationScopeKey,
		value:        extensionStorageValueRequired,
	},
	OperationMerge: {
		allowedInput: true,
		scope:        extensionStorageOperationScopeKey,
		value:        extensionStorageValueRequired,
	},
	OperationAppend: {
		allowedInput: true,
		scope:        extensionStorageOperationScopeKey,
		value:        extensionStorageValueRequired,
	},
	OperationRemove: {
		scope: extensionStorageOperationScopeKey,
		value: extensionStorageValueForbidden,
	},
	OperationClear: {
		scope: extensionStorageOperationScopeArea,
		value: extensionStorageValueForbidden,
	},
}

type extensionStorageOperationScope uint8

const (
	extensionStorageOperationScopeKey extensionStorageOperationScope = iota
	extensionStorageOperationScopeArea
)

type extensionStorageValueRequirement uint8

const (
	extensionStorageValueRequired extensionStorageValueRequirement = iota
	extensionStorageValueForbidden
)

// SettingsSource is an in-memory settings document with a diagnostic name.
type SettingsSource struct {
	Name string
	Data []byte
}

// ApplyOptions controls one extension-storage application run.
type ApplyOptions struct {
	ProfileDir         string
	Settings           []string
	SettingsSource     []SettingsSource
	ExtensionIDAliases map[string]string
	Input              Input
}

// Input supplies runtime values referenced by settings documents.
type Input struct {
	ExtensionValues map[string]any `json:"extension_values"`
}

// Settings is the public JSON document accepted by
// extension_settings.files and --settings. The legacy local/sync fields remain
// useful for bulk top-level writes, while Operations supports precise mutation
// of JSON documents stored beneath individual chrome.storage keys.
type Settings struct {
	Schema        string         `json:"$schema,omitempty"`
	SchemaVersion int            `json:"schema_version,omitzero"`
	Name          string         `json:"name,omitempty"`
	Description   string         `json:"description,omitempty"`
	Local         []Entry        `json:"local,omitempty"`
	Sync          []Entry        `json:"sync,omitempty"`
	LocalAppend   []Entry        `json:"local_append,omitempty"`
	SyncAppend    []Entry        `json:"sync_append,omitempty"`
	Operations    []Operation    `json:"operations,omitempty"`
	Inputs        []InputBinding `json:"inputs,omitempty"`
}

// Entry provides top-level storage values for one extension.
type Entry struct {
	ID     string         `json:"id"`
	Values map[string]any `json:"values"`
}

// Operation describes one direct storage mutation.
type Operation struct {
	ID        string         `json:"id"`
	Area      Area           `json:"area"`
	Key       string         `json:"key,omitempty"`
	Operation OperationKind  `json:"operation,omitempty"`
	Path      string         `json:"path,omitempty"`
	Encoding  Encoding       `json:"encoding,omitempty"`
	Value     jsontext.Value `json:"value,omitzero"`
}

// InputBinding maps a named runtime value to an extension storage target.
type InputBinding struct {
	Name      string        `json:"name"`
	Area      Area          `json:"area"`
	ID        string        `json:"id"`
	Key       string        `json:"key"`
	Path      string        `json:"path,omitempty"`
	Encoding  Encoding      `json:"encoding,omitempty"`
	Operation OperationKind `json:"operation,omitempty"`
}

type parsedSettingsSource struct {
	Name     string
	Settings Settings
}

type extensionStorageEntryGroup struct {
	name      string
	area      Area
	operation OperationKind
	entries   []Entry
}

type extensionStorageTarget struct {
	Area Area
	ID   string
}

type extensionStoragePlan struct {
	extensionStorageTarget
	Mutations []storageMutation
}

type extensionStoragePlanBuilder struct {
	aliases map[string]string
	plans   []extensionStoragePlan
	indices map[extensionStorageTarget]int
}

type storageMutation struct {
	Source    string
	Key       string
	Path      string
	Operation OperationKind
	Encoding  Encoding
	Value     any
}

type storageValueMutation struct {
	value   any
	changed bool
	remove  bool
}
