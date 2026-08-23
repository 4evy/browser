package extensionstorage

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/4evy/browser/extensions"
	"github.com/4evy/browser/internal/jsonutil"
)

// ValidateFiles validates extension-storage settings without writing a profile.
func ValidateFiles(paths []string) error {
	sources, err := loadSettingsSources(nil, paths)
	if err != nil {
		return err
	}
	_, err = parseSettingsSources(sources)
	return err
}

func loadSettingsSources(inline []SettingsSource, paths []string) ([]SettingsSource, error) {
	sources := slices.Clone(inline)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read extension settings file %s: %w", path, err)
		}
		sources = append(sources, SettingsSource{Name: path, Data: data})
	}
	return sources, nil
}

func parseSettingsSources(sources []SettingsSource) ([]parsedSettingsSource, error) {
	parsed := make([]parsedSettingsSource, 0, len(sources))
	var errs []error
	for _, source := range sources {
		settings, err := jsonutil.Strict.Decode[Settings](
			bytes.NewReader(source.Data),
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("parse extension settings file %s: %w", source.Name, err))
			continue
		}
		if err := settings.validate(); err != nil {
			errs = append(errs, fmt.Errorf("validate extension settings file %s: %w", source.Name, err))
			continue
		}
		parsed = append(parsed, parsedSettingsSource{Name: source.Name, Settings: settings})
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return parsed, nil
}

func (settings Settings) validate() error {
	var errs []error
	if settings.SchemaVersion != 0 && settings.SchemaVersion != extensionSettingsSchemaVersion {
		errs = append(errs, fmt.Errorf(
			"schema_version must be %d when present, got %d",
			extensionSettingsSchemaVersion,
			settings.SchemaVersion,
		))
	}
	for _, group := range settings.entryGroups() {
		for index, entry := range group.entries {
			errs = append(errs, validateStorageEntry(
				group.name,
				index,
				entry,
				group.operation == OperationAppend,
			))
		}
	}
	for index, operation := range settings.Operations {
		errs = append(errs, operation.validate(index))
	}
	for index, input := range settings.Inputs {
		errs = append(errs, input.validate(index))
	}
	return errors.Join(errs...)
}

func (settings Settings) entryGroups() []extensionStorageEntryGroup {
	return []extensionStorageEntryGroup{
		{
			name:      "local",
			area:      AreaLocal,
			operation: OperationSet,
			entries:   settings.Local,
		},
		{
			name:      "sync",
			area:      AreaSync,
			operation: OperationSet,
			entries:   settings.Sync,
		},
		{
			name:      "local_append",
			area:      AreaLocal,
			operation: OperationAppend,
			entries:   settings.LocalAppend,
		},
		{
			name:      "sync_append",
			area:      AreaSync,
			operation: OperationAppend,
			entries:   settings.SyncAppend,
		},
	}
}

func validateStorageEntry(
	group string,
	index int,
	entry Entry,
	appendValues bool,
) error {
	var errs []error
	if !extensions.ValidExtensionID(entry.ID) {
		errs = append(errs, fmt.Errorf("%s[%d].id is not a valid extension ID", group, index))
	}
	if entry.Values == nil {
		errs = append(errs, fmt.Errorf("%s[%d].values is required", group, index))
	}
	for _, key := range slices.Sorted(maps.Keys(entry.Values)) {
		value := entry.Values[key]
		if key == "" {
			errs = append(errs, fmt.Errorf("%s[%d].values contains an empty key", group, index))
		}
		if appendValues {
			if _, ok := value.([]any); !ok {
				errs = append(errs, fmt.Errorf(
					"%s[%d].values[%q] must be an array",
					group,
					index,
					key,
				))
			}
		}
	}
	return errors.Join(errs...)
}

func (operation Operation) validate(index int) error {
	prefix := fmt.Sprintf("operations[%d]", index)
	errs := []error{validateStorageMutationTarget(
		prefix,
		operation.ID,
		operation.Area,
		operation.Encoding,
	)}
	kind := operation.Operation.normalized()
	spec, valid := kind.spec()
	if !valid {
		errs = append(errs, fmt.Errorf(
			"%s.operation must be %s",
			prefix,
			extensionStorageOperationChoices,
		))
		return errors.Join(errs...)
	}
	errs = append(errs, operation.validateShape(prefix, kind, spec))
	return errors.Join(errs...)
}

func (operation Operation) validateShape(
	prefix string,
	kind OperationKind,
	spec extensionStorageOperationSpec,
) error {
	if spec.scope == extensionStorageOperationScopeArea {
		if operation.Key != "" || operation.Path != "" || len(operation.Value) != 0 {
			return fmt.Errorf(
				"%s %s must not specify key, path, or value",
				prefix,
				kind,
			)
		}
		return nil
	}
	var errs []error
	if operation.Key == "" {
		errs = append(errs, fmt.Errorf("%s.key is required", prefix))
	}
	errs = append(errs, operation.validateValue(prefix, kind, spec.value))
	return errors.Join(errs...)
}

func (operation Operation) validateValue(
	prefix string,
	kind OperationKind,
	requirement extensionStorageValueRequirement,
) error {
	if requirement == extensionStorageValueForbidden {
		if len(operation.Value) != 0 {
			return fmt.Errorf("%s %s must not specify value", prefix, kind)
		}
		return nil
	}
	if len(operation.Value) == 0 {
		return fmt.Errorf("%s.value is required", prefix)
	}
	value, err := rawJSONValue(operation.Value)
	if err != nil {
		return fmt.Errorf("%s.value: %w", prefix, err)
	}
	return validateMutationValue(prefix, kind, value)
}

func (input InputBinding) validate(index int) error {
	prefix := fmt.Sprintf("inputs[%d]", index)
	errs := []error{validateStorageMutationTarget(
		prefix,
		input.ID,
		input.Area,
		input.Encoding,
	)}
	if input.Name == "" {
		errs = append(errs, fmt.Errorf("%s.name is required", prefix))
	}
	if input.Key == "" {
		errs = append(errs, fmt.Errorf("%s.key is required", prefix))
	}
	if !input.Operation.normalized().validForInput() {
		errs = append(errs, fmt.Errorf(
			"%s.operation must be %s",
			prefix,
			extensionStorageInputOpChoices,
		))
	}
	return errors.Join(errs...)
}

func validateStorageMutationTarget(
	prefix,
	id string,
	area Area,
	encoding Encoding,
) error {
	var errs []error
	if !extensions.ValidExtensionID(id) {
		errs = append(errs, fmt.Errorf("%s.id is not a valid extension ID", prefix))
	}
	if !area.valid() {
		errs = append(errs, fmt.Errorf(
			"%s.area must be %s",
			prefix,
			extensionStorageAreaChoices,
		))
	}
	if !encoding.normalized().valid() {
		errs = append(errs, fmt.Errorf(
			"%s.encoding must be %s",
			prefix,
			extensionStorageEncodingChoices,
		))
	}
	return errors.Join(errs...)
}

func validateMutationValue(
	prefix string,
	operation OperationKind,
	value any,
) error {
	switch operation {
	case OperationSet,
		OperationRemove,
		OperationClear:
	case OperationMerge:
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("%s.value must be an object for merge", prefix)
		}
	case OperationAppend:
		if _, ok := value.([]any); !ok {
			return fmt.Errorf("%s.value must be an array for append", prefix)
		}
	}
	return nil
}

func (area Area) valid() bool {
	switch area {
	case AreaLocal, AreaSync:
		return true
	default:
		return false
	}
}

func (encoding Encoding) normalized() Encoding {
	if encoding == "" {
		return EncodingJSON
	}
	return encoding
}

func (encoding Encoding) valid() bool {
	switch encoding {
	case EncodingJSON, EncodingLZStringURI:
		return true
	default:
		return false
	}
}

func (operation OperationKind) normalized() OperationKind {
	if operation == "" {
		return OperationSet
	}
	return operation
}

func (operation OperationKind) validForInput() bool {
	spec, valid := operation.spec()
	return valid && spec.allowedInput
}

func (operation OperationKind) spec() (extensionStorageOperationSpec, bool) {
	switch operation {
	case OperationSet,
		OperationMerge,
		OperationAppend:
		return extensionStorageOperationSpec{
			allowedInput: true,
			scope:        extensionStorageOperationScopeKey,
			value:        extensionStorageValueRequired,
		}, true
	case OperationRemove:
		return extensionStorageOperationSpec{
			scope: extensionStorageOperationScopeKey,
			value: extensionStorageValueForbidden,
		}, true
	case OperationClear:
		return extensionStorageOperationSpec{
			scope: extensionStorageOperationScopeArea,
			value: extensionStorageValueForbidden,
		}, true
	default:
		return extensionStorageOperationSpec{}, false
	}
}

func resolveExtensionID(aliases map[string]string, id string) string {
	if alias := aliases[id]; alias != "" {
		return alias
	}
	return id
}
