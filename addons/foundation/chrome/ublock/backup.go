package ublock

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"os"
	"strings"

	jsonutil "github.com/4evy/tweezle/internal/json"
)

// originBackup matches uBlock's Settings > Back up to file export
// Source: gorhill/uBlock/src/js/messaging.js at backupUserData
type originBackup struct {
	TimeStamp              int64          `json:"timeStamp"`
	Version                string         `json:"version"`
	UserSettings           map[string]any `json:"userSettings"`
	HiddenSettings         map[string]any `json:"hiddenSettings"`
	SelectedFilterLists    []string       `json:"selectedFilterLists"`
	Whitelist              []string       `json:"whitelist"`
	DynamicFilteringString string         `json:"dynamicFilteringString"`
	URLFilteringString     string         `json:"urlFilteringString"`
	HostnameSwitchesString string         `json:"hostnameSwitchesString"`
	UserFilters            jsontext.Value `json:"userFilters"`
}

func loadOriginSettings(file string) (OriginSettings, error) {
	if file == "" {
		return OriginSettings{}, nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return OriginSettings{}, fmt.Errorf("read settings_file: %w", err)
	}
	fields, err := jsonutil.Strict.Decode[map[string]jsontext.Value](bytes.NewReader(data))
	if err != nil {
		return OriginSettings{}, wrap("settings_file", err)
	}
	if _, backup := fields["userSettings"]; !backup {
		settings, err := jsonutil.Strict.Decode[OriginSettings](bytes.NewReader(data))
		return settings, wrap("settings_file", err)
	}
	backup, err := jsonutil.Strict.Decode[originBackup](bytes.NewReader(data))
	if err != nil {
		return OriginSettings{}, wrap("settings_file", err)
	}
	if backup.UserSettings == nil || backup.SelectedFilterLists == nil || backup.Whitelist == nil {
		return OriginSettings{}, fmt.Errorf("settings_file: uBlock backup must contain userSettings, selectedFilterLists, and whitelist")
	}
	filters, err := backupFilterText(backup.UserFilters)
	if err != nil {
		return OriginSettings{}, wrap("settings_file.userFilters", err)
	}
	rules := strings.Join([]string{
		backup.DynamicFilteringString,
		backup.URLFilteringString,
		backup.HostnameSwitchesString,
	}, "\n")
	return OriginSettings{
		UserSettings: backup.UserSettings, HiddenSettings: backup.HiddenSettings,
		FilterLists: backup.SelectedFilterLists, TrustedSites: backup.Whitelist,
		UserFilters: &filters, Rules: &rules,
	}, nil
}

func backupFilterText(data jsontext.Value) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	switch data.Kind() {
	case '[':
		var lines []string
		if err := json.Unmarshal(data, &lines); err != nil {
			return "", fmt.Errorf("must be an array of strings: %w", err)
		}
		return strings.Join(lines, "\n"), nil
	case '"':
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return "", fmt.Errorf("must be a string: %w", err)
		}
		return value, nil
	default:
		return "", fmt.Errorf("must be a string or an array of strings")
	}
}
