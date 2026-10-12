// Package ublock defines native uBlock Origin and uBlock Origin Lite settings
package ublock

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/4evy/tweezle/addons/foundation/chrome/extensions"
	"github.com/4evy/tweezle/addons/foundation/chrome/storage"
	jsonutil "github.com/4evy/tweezle/internal/json"
)

const (
	OriginID = "fkgkibajhfbepljeaefdnfnegdcjomkh"
	LiteID   = "ddkjiahejlhfcafbddmgiahcphecmpfh"
	// Public key from the author's signed Chromium release (uBO 1.75.0)
	// Its stable ID is independent of the installation directory
	OriginManifestKey = "MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAsgdkJHEX8xHAytYy3Rihqn5FoU/cbPKhoorkCCsgF8HR2y2OSWGM1Ojrmnr0ebgM9WA1pl1hr1CmOH7DjgQVKRhzBjK7/Zb6RVJNPVGEQvV9CdCUwOTKsu1qQGRbjm9Z/DkYxgu6B2sLo0ZpQ/IBsmBvs+FGR4CqrWra8GZPwn7n3FibeoxcArWiAx85N2Oyiaef2Geytoog4hS+I5Fs3ymKkEeTYM3tzeC0U5nZ010LCnlQe0cQ3UDOro8VzLosuhaxAsrPFErIOfIUfvV3sNhQJrySqgii9Xv6RWT8TI3pHL1yjevKKTxNb2VbPlTOi5MyzPowWV8hHJEOkwq2dQIDAQAB"
	StoreUpdateURL    = "https://clients2.google.com/service/update2/crx"
)

//go:embed origin.js
var originScript string

type Config struct {
	Origin *OriginConfig `toml:"origin"`
	Lite   *LiteConfig   `toml:"lite"`
}

// Installation controls are shared by both variants. A table opts in to
// installation unless install is false; settings_file is relative to the TOML
// file and inline settings override its fields
// ID selects an existing variant when install is false
// ExtensionDir is needed only for Origin runtime settings on an existing copy
// Use builtin for a browser component that loads even in a temporary profile
type Installation struct {
	Install      *bool          `toml:"install"`
	ID           string         `toml:"id"`
	SettingsFile string         `toml:"settings_file"`
	ExtensionDir string         `toml:"extension_dir"`
	Builtin      bool           `toml:"builtin"`
	Managed      map[string]any `toml:"managed"`
}

type OriginConfig struct {
	Installation   `toml:",inline"`
	OriginSettings `toml:",inline"`
}

type OriginSettings struct {
	UserSettings   map[string]any `toml:"user_settings" json:"user_settings,omitempty"`
	HiddenSettings map[string]any `toml:"hidden_settings" json:"hidden_settings,omitempty"`
	FilterLists    []string       `toml:"filter_lists" json:"filter_lists,omitzero"`
	TrustedSites   []string       `toml:"trusted_sites" json:"trusted_sites,omitzero"`
	UserFilters    *string        `toml:"user_filters" json:"user_filters,omitzero"`
	Rules          *string        `toml:"rules" json:"rules,omitzero"`
}

type LiteConfig struct {
	Installation `toml:",inline"`
	LiteSettings `toml:",inline"`
}

type LiteSettings struct {
	Rulesets         []string `toml:"rulesets" json:"rulesets,omitempty"`
	AutoReload       *bool    `toml:"auto_reload" json:"auto_reload,omitempty"`
	ShowBlockedCount *bool    `toml:"show_blocked_count" json:"show_blocked_count,omitempty"`
	StrictBlockMode  *bool    `toml:"strict_block_mode" json:"strict_block_mode,omitempty"`
	PopupBlockMode   *bool    `toml:"popup_block_mode" json:"popup_block_mode,omitempty"`
	DeveloperMode    *bool    `toml:"developer_mode" json:"developer_mode,omitempty"`
	// FilteringModes replaces the complete mode assignment, including the
	// all-urls default. Stronger modes need host permission granted in the UI
	FilteringModes *FilteringModes `toml:"filtering_modes" json:"filtering_modes,omitempty"`
}

type FilteringModes struct {
	None     []string `toml:"none" json:"none"`
	Basic    []string `toml:"basic" json:"basic"`
	Optimal  []string `toml:"optimal" json:"optimal"`
	Complete []string `toml:"complete" json:"complete"`
}

func (installation Installation) ShouldInstall() bool {
	return installation.Install == nil || *installation.Install
}

func (installation Installation) Validate() error {
	if installation.ID != "" && !extensions.ValidExtensionID(installation.ID) {
		return errors.New("id must be a Chromium extension ID")
	}
	if installation.ShouldInstall() && (installation.ID != "" || installation.ExtensionDir != "" || installation.Builtin) {
		return errors.New("id, extension_dir, and builtin require install = false")
	}
	if installation.Builtin && installation.ID == "" {
		return errors.New("builtin requires the browser component id")
	}
	if installation.ExtensionDir != "" {
		id, err := extensions.UnpackedManifestID(installation.ExtensionDir)
		if err != nil {
			return fmt.Errorf("extension_dir: %w", err)
		}
		if installation.ID != "" && installation.ID != id {
			return fmt.Errorf("extension_dir has ID %s, want %s", id, installation.ID)
		}
	}
	if installation.Builtin && installation.ExtensionDir != "" {
		return errors.New("builtin and extension_dir cannot be combined")
	}
	return nil
}

func (config Config) Validate() error {
	var errs []error
	if config.Origin != nil {
		_, err := config.Origin.Settings()
		errs = append(errs, wrap("ublock.origin", errors.Join(config.Origin.Validate(), err)))
	}
	if config.Lite != nil {
		settings, err := config.Lite.Settings()
		errs = append(errs, wrap("ublock.lite", errors.Join(config.Lite.Installation.Validate(), err, settings.Validate())))
		if config.Lite.Builtin || config.Lite.ExtensionDir != "" {
			errs = append(errs, errors.New("ublock.lite uses persistent storage; builtin and extension_dir are only supported by ublock.origin"))
		}
	}
	return errors.Join(errs...)
}

func wrap(name string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", name, err)
}

func (config OriginConfig) Settings() (OriginSettings, error) {
	settings, err := loadOriginSettings(config.SettingsFile)
	if err != nil {
		return settings, err
	}
	settings.UserSettings = overlay(settings.UserSettings, config.UserSettings)
	settings.HiddenSettings = overlay(settings.HiddenSettings, config.HiddenSettings)
	if config.FilterLists != nil {
		settings.FilterLists = config.FilterLists
	}
	if config.TrustedSites != nil {
		settings.TrustedSites = config.TrustedSites
	}
	if config.UserFilters != nil {
		settings.UserFilters = config.UserFilters
	}
	if config.Rules != nil {
		settings.Rules = config.Rules
	}
	for name, values := range map[string]map[string]any{"user_settings": settings.UserSettings, "hidden_settings": settings.HiddenSettings} {
		for key, value := range values {
			if key == "" {
				return settings, fmt.Errorf("%s contains an empty key", name)
			}
			if name == "user_settings" && key == "importedLists" {
				data, err := json.Marshal(value)
				var lists []string
				if err != nil || json.Unmarshal(data, &lists) != nil || lists == nil {
					return settings, errors.New("user_settings.importedLists must be an array of strings")
				}
				continue
			}
			data, err := json.Marshal(value)
			if err != nil {
				return settings, fmt.Errorf("%s.%s: %w", name, key, err)
			}
			switch jsontext.Value(data).Kind() {
			case '"', '0', 't', 'f':
				continue
			case '[':
				if name == "user_settings" {
					continue
				}
			}
			return settings, fmt.Errorf("%s.%s must be a scalar (user_settings also accepts arrays)", name, key)
		}
	}
	return settings, nil
}

func (config LiteConfig) Settings() (LiteSettings, error) {
	settings, err := loadSettings[LiteSettings](config.SettingsFile)
	if err != nil {
		return settings, err
	}
	// Pointer and slice presence distinguish omission from explicit false/empty
	target, inline := reflect.ValueOf(&settings).Elem(), reflect.ValueOf(config.LiteSettings)
	for index := range target.NumField() {
		if !inline.Field(index).IsNil() {
			target.Field(index).Set(inline.Field(index))
		}
	}
	return settings, settings.Validate()
}

func (settings LiteSettings) Validate() error {
	if settings.FilteringModes == nil {
		return nil
	}
	modes := settings.FilteringModes
	seen := map[string]bool{}
	defaults := 0
	for _, hostnames := range [][]string{modes.None, modes.Basic, modes.Optimal, modes.Complete} {
		for _, hostname := range hostnames {
			if hostname == "" || strings.ContainsAny(hostname, " /:\t\r\n") {
				return fmt.Errorf("invalid filtering hostname %q", hostname)
			}
			if seen[hostname] {
				return fmt.Errorf("filtering hostname %q occurs more than once", hostname)
			}
			seen[hostname] = true
			if hostname == "all-urls" {
				defaults++
			}
		}
	}
	if defaults != 1 {
		return errors.New("filtering_modes must place all-urls in exactly one mode")
	}
	return nil
}

func loadSettings[T any](file string) (T, error) {
	var settings T
	if file == "" {
		return settings, nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return settings, fmt.Errorf("read settings_file: %w", err)
	}
	settings, err = jsonutil.Strict.Decode[T](bytes.NewReader(data))
	return settings, wrap("settings_file", err)
}

func overlay(defaults, overrides map[string]any) map[string]any {
	if len(defaults) == 0 && len(overrides) == 0 {
		return nil
	}
	out := maps.Clone(defaults)
	if out == nil {
		out = map[string]any{}
	}
	maps.Copy(out, overrides)
	return out
}

func (config Config) AddExtensions(catalog extensions.Catalog) extensions.Catalog {
	catalog.ZIP = slices.Clone(catalog.ZIP)
	catalog.ChromeStore = slices.Clone(catalog.ChromeStore)
	if config.Origin != nil && config.Origin.ShouldInstall() {
		catalog.ZIP = append(catalog.ZIP, extensions.ZIPExtension{
			ID: OriginID, Name: "uBlock Origin", UpdatePolicy: extensions.UpdatePolicyLatest,
			Repository: "gorhill/uBlock", AssetTemplate: "uBlock0_{tag}.chromium.zip",
			ArchiveRoot: "uBlock0.chromium", LoadUnpacked: true, ManifestKey: OriginManifestKey,
		})
	}
	if config.Lite != nil && config.Lite.ShouldInstall() {
		if catalog.ChromeStoreUpdateURL == "" {
			catalog.ChromeStoreUpdateURL = StoreUpdateURL
		}
		catalog.ChromeStore = append(catalog.ChromeStore, extensions.ChromeStoreExtension{ID: LiteID, Name: "uBlock Origin Lite"})
	}
	return catalog
}

func (config OriginConfig) InstalledID() string {
	if config.ID != "" {
		return config.ID
	}
	if config.ExtensionDir != "" {
		// Validation has already checked this manifest before compiling settings
		if id, err := extensions.UnpackedManifestID(config.ExtensionDir); err == nil {
			return id
		}
	}
	return OriginID
}
func (config LiteConfig) InstalledID() string {
	if config.ID != "" {
		return config.ID
	}
	return LiteID
}

// Script uses the extension's dashboard API so it updates assets, caches, and
// settings together. It rejects unrecognized settings before applying changes
func (config OriginConfig) Script() (string, error) {
	settings, err := config.Settings()
	if err != nil {
		return "", err
	}
	if reflect.ValueOf(settings).IsZero() {
		return "", nil
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return "", err
	}
	return "const settings = " + string(data) + ";\n" + originScript, nil
}

func (config LiteConfig) StorageSource(manifestPath string) (storage.SettingsSource, error) {
	settings, err := config.Settings()
	if err != nil {
		return storage.SettingsSource{}, err
	}
	values := map[string]any{}
	for key, value := range map[string]*bool{
		"autoReload": settings.AutoReload, "showBlockedCount": settings.ShowBlockedCount,
		"strictBlockMode": settings.StrictBlockMode, "popupBlockMode": settings.PopupBlockMode,
		"developerMode": settings.DeveloperMode,
	} {
		if value != nil {
			values[key] = *value
		}
	}
	if settings.Rulesets != nil {
		manifest, err := loadLiteManifest(manifestPath)
		if err != nil {
			return storage.SettingsSource{}, err
		}
		available := map[string]bool{}
		defaults := []string{}
		for _, ruleset := range manifest.Rules.Resources {
			available[ruleset.ID] = true
			if ruleset.Enabled {
				defaults = append(defaults, ruleset.ID)
			}
		}
		selected := []string{}
		for _, id := range settings.Rulesets {
			if id == "default" && !available[id] {
				selected = append(selected, defaults...)
				continue
			}
			if !available[id] {
				return storage.SettingsSource{}, fmt.Errorf("uBlock Origin Lite ruleset %q is not available in version %s", id, manifest.Version)
			}
			selected = append(selected, id)
		}
		values["enabledRulesets"] = selected
		// Prevent first-run/version initialization from adding defaults over the
		// explicit selection. Future extension upgrades retain their migrations
		values["version"] = manifest.Version
		values["defaultRulesetIds"] = defaults
	}
	document := storage.Settings{}
	if defaults, ok := values["defaultRulesetIds"]; ok {
		document.Local = append(document.Local, storage.Entry{ID: config.InstalledID(), Values: map[string]any{"defaultRulesetIds": defaults}})
		delete(values, "defaultRulesetIds")
	}
	if len(values) > 0 {
		data, err := json.Marshal(values)
		if err != nil {
			return storage.SettingsSource{}, err
		}
		document.Operations = append(document.Operations, storage.Operation{
			ID: config.InstalledID(), Area: storage.AreaLocal, Key: "rulesetConfig", Operation: storage.OperationMerge, Value: data,
		})
	}
	if settings.FilteringModes != nil {
		document.Local = append(document.Local, storage.Entry{ID: config.InstalledID(), Values: map[string]any{"filteringModeDetails": settings.FilteringModes}})
	}
	data, err := json.Marshal(document)
	return storage.SettingsSource{Name: "ublock.lite", Data: data}, err
}

func (config Config) ManagedPolicies(aliases map[string]string, updateURL string) map[string]any {
	if updateURL == "" {
		updateURL = StoreUpdateURL
	}
	policies := map[string]any{}
	managed := map[string]any{}
	installs := map[string]any{}
	add := func(id string, installation Installation, lite bool) {
		if alias := aliases[id]; alias != "" {
			id = alias
		}
		if len(installation.Managed) > 0 {
			managed[id] = maps.Clone(installation.Managed)
		}
		if installation.ShouldInstall() && lite {
			installs[id] = map[string]any{"installation_mode": "force_installed", "update_url": updateURL}
		}
	}
	if config.Origin != nil {
		add(config.Origin.InstalledID(), config.Origin.Installation, false)
	}
	if config.Lite != nil {
		add(config.Lite.InstalledID(), config.Lite.Installation, true)
	}
	if len(managed) > 0 {
		policies["3rdparty"] = map[string]any{"extensions": managed}
	}
	if len(installs) > 0 {
		policies["ExtensionSettings"] = installs
	}
	return policies
}

type liteManifest struct {
	Version string `json:"version"`
	Rules   struct {
		Resources []struct {
			ID      string `json:"id"`
			Enabled bool   `json:"enabled"`
		} `json:"rule_resources"`
	} `json:"declarative_net_request"`
}

func loadLiteManifest(path string) (manifest liteManifest, err error) {
	if path == "" {
		return manifest, errors.New("uBlock Origin Lite rulesets require an installed manifest; run tweezle apply first or install Lite in this profile")
	}
	var reader io.ReadCloser
	if filepath.Ext(path) == ".crx" {
		archive, openErr := zip.OpenReader(path)
		if openErr != nil {
			return manifest, fmt.Errorf("read Lite CRX: %w", openErr)
		}
		defer func() { err = errors.Join(err, archive.Close()) }()
		reader, err = archive.Open("manifest.json")
	} else {
		reader, err = os.Open(path)
	}
	if err != nil {
		return manifest, err
	}
	defer func() { err = errors.Join(err, reader.Close()) }()
	const maximumManifestSize = 1 << 20
	data, err := io.ReadAll(io.LimitReader(reader, maximumManifestSize+1))
	if err != nil {
		return manifest, fmt.Errorf("read Lite manifest: %w", err)
	}
	if len(data) > maximumManifestSize {
		return manifest, errors.New("lite manifest exceeds 1 MiB")
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("read Lite manifest: %w", err)
	}
	if manifest.Version == "" || len(manifest.Rules.Resources) == 0 {
		return manifest, errors.New("uBlock Origin Lite manifest must declare a version and static rulesets")
	}
	return manifest, nil
}
