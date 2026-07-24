package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"syscall"

	"github.com/4evy/browser/extensions"
	"github.com/4evy/browser/internal/fileutil"
	"github.com/Jeffail/gabs/v2"
	lzstring "github.com/daku10/go-lz-string"
	"github.com/syndtr/goleveldb/leveldb"
	leveldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

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

type ExtensionStorageArea string

const (
	ExtensionStorageAreaLocal ExtensionStorageArea = "local"
	ExtensionStorageAreaSync  ExtensionStorageArea = "sync"
)

type ExtensionStorageEncoding string

const (
	ExtensionStorageEncodingJSON        ExtensionStorageEncoding = "json"
	ExtensionStorageEncodingLZStringURI ExtensionStorageEncoding = "json-lz-string-uri"
)

type ExtensionStorageOperationKind string

const (
	ExtensionStorageOperationSet    ExtensionStorageOperationKind = "set"
	ExtensionStorageOperationMerge  ExtensionStorageOperationKind = "merge"
	ExtensionStorageOperationAppend ExtensionStorageOperationKind = "append"
	ExtensionStorageOperationRemove ExtensionStorageOperationKind = "remove"
	ExtensionStorageOperationClear  ExtensionStorageOperationKind = "clear"
)

type extensionStorageOperationSpec struct {
	allowedInput bool
	scope        extensionStorageOperationScope
	value        extensionStorageValueRequirement
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

type SettingsSource struct {
	Name string
	Data []byte
}

type ApplyOptions struct {
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

// ExtensionStorageSettings is the public JSON document accepted by
// extension_settings.files and --settings. The legacy local/sync fields remain
// useful for bulk top-level writes, while Operations supports precise mutation
// of JSON documents stored beneath individual chrome.storage keys.
type ExtensionStorageSettings struct {
	Schema        string                      `json:"$schema,omitempty"`
	SchemaVersion int                         `json:"schema_version,omitempty"`
	Name          string                      `json:"name,omitempty"`
	Description   string                      `json:"description,omitempty"`
	Local         []ExtensionStorageEntry     `json:"local,omitempty"`
	Sync          []ExtensionStorageEntry     `json:"sync,omitempty"`
	LocalAppend   []ExtensionStorageEntry     `json:"local_append,omitempty"`
	SyncAppend    []ExtensionStorageEntry     `json:"sync_append,omitempty"`
	Operations    []ExtensionStorageOperation `json:"operations,omitempty"`
	Inputs        []ExtensionStorageInput     `json:"inputs,omitempty"`
}

type ExtensionStorageEntry struct {
	ID     string         `json:"id"`
	Values map[string]any `json:"values"`
}

type ExtensionStorageOperation struct {
	ID        string                        `json:"id"`
	Area      ExtensionStorageArea          `json:"area"`
	Key       string                        `json:"key,omitempty"`
	Operation ExtensionStorageOperationKind `json:"operation,omitempty"`
	Path      string                        `json:"path,omitempty"`
	Encoding  ExtensionStorageEncoding      `json:"encoding,omitempty"`
	Value     json.RawMessage               `json:"value,omitempty"`
}

type ExtensionStorageInput struct {
	Name      string                        `json:"name"`
	Area      ExtensionStorageArea          `json:"area"`
	ID        string                        `json:"id"`
	Key       string                        `json:"key"`
	Path      string                        `json:"path,omitempty"`
	Encoding  ExtensionStorageEncoding      `json:"encoding,omitempty"`
	Operation ExtensionStorageOperationKind `json:"operation,omitempty"`
}

type parsedSettingsSource struct {
	Name     string
	Settings ExtensionStorageSettings
}

type extensionStorageEntryGroup struct {
	name      string
	area      ExtensionStorageArea
	operation ExtensionStorageOperationKind
	entries   []ExtensionStorageEntry
}

type extensionStorageTarget struct {
	Area ExtensionStorageArea
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
	Operation ExtensionStorageOperationKind
	Encoding  ExtensionStorageEncoding
	Value     any
}

type storageValueMutation struct {
	value   any
	changed bool
	remove  bool
}

func ApplyExtensionSettings(ctx context.Context, options ApplyOptions) error {
	if err := validateExtensionIDAliases(options.ExtensionIDAliases); err != nil {
		return err
	}
	sources, err := loadSettingsSources(options.SettingsSource, options.Settings)
	if err != nil {
		return err
	}
	parsed, err := parseSettingsSources(sources)
	if err != nil {
		return err
	}
	plans, err := buildExtensionStoragePlans(ctx, parsed, options)
	if err != nil {
		return err
	}
	for _, plan := range plans {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := applyExtensionStoragePlan(options.ProfileDir, plan); err != nil {
			return err
		}
	}
	return nil
}

func buildExtensionStoragePlans(
	ctx context.Context,
	sources []parsedSettingsSource,
	options ApplyOptions,
) ([]extensionStoragePlan, error) {
	builder := extensionStoragePlanBuilder{
		aliases: options.ExtensionIDAliases,
		indices: map[extensionStorageTarget]int{},
	}
	for _, source := range sources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := builder.addSource(source, options.Input); err != nil {
			return nil, err
		}
	}
	return builder.plans, nil
}

func (builder *extensionStoragePlanBuilder) addSource(
	source parsedSettingsSource,
	input ApplyInput,
) error {
	builder.addEntryGroups(source)
	if err := builder.addOperations(source); err != nil {
		return err
	}
	return builder.addInputs(source, input)
}

func (builder *extensionStoragePlanBuilder) addEntryGroups(source parsedSettingsSource) {
	for _, group := range source.Settings.entryGroups() {
		for _, entry := range group.entries {
			for _, key := range slices.Sorted(maps.Keys(entry.Values)) {
				builder.add(group.area, entry.ID, storageMutation{
					Source:    source.Name,
					Key:       key,
					Operation: group.operation,
					Encoding:  ExtensionStorageEncodingJSON,
					Value:     entry.Values[key],
				})
			}
		}
	}
}

func (builder *extensionStoragePlanBuilder) addOperations(source parsedSettingsSource) error {
	for _, operation := range source.Settings.Operations {
		value, err := rawJSONValue(operation.Value)
		if err != nil {
			return fmt.Errorf("read operation from %s: %w", source.Name, err)
		}
		builder.add(operation.Area, operation.ID, storageMutation{
			Source:    source.Name,
			Key:       operation.Key,
			Path:      operation.Path,
			Operation: operation.Operation.normalized(),
			Encoding:  operation.Encoding.normalized(),
			Value:     value,
		})
	}
	return nil
}

func (builder *extensionStoragePlanBuilder) addInputs(
	source parsedSettingsSource,
	input ApplyInput,
) error {
	for _, setting := range source.Settings.Inputs {
		value, ok := input.ExtensionValues[setting.Name]
		if !ok {
			continue
		}
		kind := setting.Operation.normalized()
		inputSource := fmt.Sprintf("input %q from %s", setting.Name, source.Name)
		if err := validateMutationValue(inputSource, kind, value); err != nil {
			return err
		}
		builder.add(setting.Area, setting.ID, storageMutation{
			Source:    inputSource,
			Key:       setting.Key,
			Path:      setting.Path,
			Operation: kind,
			Encoding:  setting.Encoding.normalized(),
			Value:     value,
		})
	}
	return nil
}

func (builder *extensionStoragePlanBuilder) add(
	area ExtensionStorageArea,
	id string,
	mutation storageMutation,
) {
	target := extensionStorageTarget{
		Area: area,
		ID:   resolveExtensionID(builder.aliases, id),
	}
	index, exists := builder.indices[target]
	if !exists {
		index = len(builder.plans)
		builder.indices[target] = index
		builder.plans = append(
			builder.plans,
			extensionStoragePlan{extensionStorageTarget: target},
		)
	}
	builder.plans[index].Mutations = append(builder.plans[index].Mutations, mutation)
}

func applyExtensionStoragePlan(profileDir string, plan extensionStoragePlan) error {
	area, err := plan.Area.directory()
	if err != nil {
		return err
	}
	return withStorage(profileDir, area, plan.ID, func(database *leveldb.DB) error {
		state, err := readExtensionStorageState(database)
		if err != nil {
			return fmt.Errorf("read %s/%s: %w", area, plan.ID, err)
		}
		original := maps.Clone(state)
		if err := applyStorageMutations(state, area, plan); err != nil {
			return err
		}
		if plan.Area == ExtensionStorageAreaSync {
			if err := validateSyncStorageState(state); err != nil {
				return fmt.Errorf("validate %s/%s: %w", area, plan.ID, err)
			}
		}
		return writeExtensionStorageState(database, original, state)
	})
}

func applyStorageMutations(
	state map[string][]byte,
	area string,
	plan extensionStoragePlan,
) error {
	for _, mutation := range plan.Mutations {
		if err := applyStorageMutation(state, area, plan.ID, mutation); err != nil {
			return err
		}
	}
	return nil
}

func applyStorageMutation(
	state map[string][]byte,
	area,
	extensionID string,
	mutation storageMutation,
) error {
	if applyDirectStorageMutation(state, mutation) {
		return nil
	}
	document, err := decodeMutationDocument(state, mutation)
	if err != nil {
		return fmt.Errorf(
			"apply %s to %s/%s/%s from %s: decode: %w",
			mutation.Operation,
			area,
			extensionID,
			mutation.Key,
			mutation.Source,
			err,
		)
	}
	document, changed, err := mutateStorageValue(
		document,
		mutation.Path,
		mutation.Operation,
		mutation.Value,
	)
	if err != nil {
		return fmt.Errorf(
			"apply %s to %s/%s/%s path %q from %s: %w",
			mutation.Operation,
			area,
			extensionID,
			mutation.Key,
			mutation.Path,
			mutation.Source,
			err,
		)
	}
	if !changed {
		return nil
	}
	state[mutation.Key], err = encodeStorageValue(document, mutation.Encoding)
	if err != nil {
		return fmt.Errorf(
			"apply %s to %s/%s/%s from %s: encode: %w",
			mutation.Operation,
			area,
			extensionID,
			mutation.Key,
			mutation.Source,
			err,
		)
	}
	return nil
}

func applyDirectStorageMutation(
	state map[string][]byte,
	mutation storageMutation,
) bool {
	switch mutation.Operation {
	case ExtensionStorageOperationClear:
		clear(state)
		return true
	case ExtensionStorageOperationRemove:
		if mutation.Path == "" {
			delete(state, mutation.Key)
			return true
		}
	case ExtensionStorageOperationSet,
		ExtensionStorageOperationMerge,
		ExtensionStorageOperationAppend:
	}
	return false
}

func decodeMutationDocument(
	state map[string][]byte,
	mutation storageMutation,
) (any, error) {
	raw, exists := state[mutation.Key]
	if exists {
		decoded, err := decodeStorageValue(raw, mutation.Encoding)
		if err != nil {
			return nil, err
		}
		return decoded, nil
	}
	if mutation.Path != "" {
		return map[string]any{}, nil
	}
	return nil, nil
}

func readExtensionStorageState(database *leveldb.DB) (map[string][]byte, error) {
	state := map[string][]byte{}
	iterator := database.NewIterator(nil, nil)
	defer iterator.Release()
	for iterator.Next() {
		state[string(iterator.Key())] = slices.Clone(iterator.Value())
	}
	if err := iterator.Error(); err != nil {
		return nil, err
	}
	return state, nil
}

func writeExtensionStorageState(
	database *leveldb.DB,
	original,
	state map[string][]byte,
) error {
	batch := new(leveldb.Batch)
	for _, key := range slices.Sorted(maps.Keys(original)) {
		if _, exists := state[key]; !exists {
			batch.Delete([]byte(key))
		}
	}
	for _, key := range slices.Sorted(maps.Keys(state)) {
		value := state[key]
		if bytes.Equal(original[key], value) {
			continue
		}
		batch.Put([]byte(key), value)
	}
	return database.Write(batch, nil)
}

func ValidateExtensionSettingsFiles(paths []string) error {
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
		var settings ExtensionStorageSettings
		if err := decodeJSONStrict(bytes.NewReader(source.Data), &settings); err != nil {
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

func (settings ExtensionStorageSettings) validate() error {
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
				group.operation == ExtensionStorageOperationAppend,
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

func (settings ExtensionStorageSettings) entryGroups() []extensionStorageEntryGroup {
	return []extensionStorageEntryGroup{
		{
			name:      "local",
			area:      ExtensionStorageAreaLocal,
			operation: ExtensionStorageOperationSet,
			entries:   settings.Local,
		},
		{
			name:      "sync",
			area:      ExtensionStorageAreaSync,
			operation: ExtensionStorageOperationSet,
			entries:   settings.Sync,
		},
		{
			name:      "local_append",
			area:      ExtensionStorageAreaLocal,
			operation: ExtensionStorageOperationAppend,
			entries:   settings.LocalAppend,
		},
		{
			name:      "sync_append",
			area:      ExtensionStorageAreaSync,
			operation: ExtensionStorageOperationAppend,
			entries:   settings.SyncAppend,
		},
	}
}

func validateStorageEntry(
	group string,
	index int,
	entry ExtensionStorageEntry,
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

func (operation ExtensionStorageOperation) validate(index int) error {
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

func (operation ExtensionStorageOperation) validateShape(
	prefix string,
	kind ExtensionStorageOperationKind,
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

func (operation ExtensionStorageOperation) validateValue(
	prefix string,
	kind ExtensionStorageOperationKind,
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

func (input ExtensionStorageInput) validate(index int) error {
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
	area ExtensionStorageArea,
	encoding ExtensionStorageEncoding,
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
	operation ExtensionStorageOperationKind,
	value any,
) error {
	switch operation {
	case ExtensionStorageOperationSet,
		ExtensionStorageOperationRemove,
		ExtensionStorageOperationClear:
	case ExtensionStorageOperationMerge:
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("%s.value must be an object for merge", prefix)
		}
	case ExtensionStorageOperationAppend:
		if _, ok := value.([]any); !ok {
			return fmt.Errorf("%s.value must be an array for append", prefix)
		}
	}
	return nil
}

func (area ExtensionStorageArea) valid() bool {
	switch area {
	case ExtensionStorageAreaLocal, ExtensionStorageAreaSync:
		return true
	default:
		return false
	}
}

func (encoding ExtensionStorageEncoding) normalized() ExtensionStorageEncoding {
	if encoding == "" {
		return ExtensionStorageEncodingJSON
	}
	return encoding
}

func (encoding ExtensionStorageEncoding) valid() bool {
	switch encoding {
	case ExtensionStorageEncodingJSON, ExtensionStorageEncodingLZStringURI:
		return true
	default:
		return false
	}
}

func (operation ExtensionStorageOperationKind) normalized() ExtensionStorageOperationKind {
	if operation == "" {
		return ExtensionStorageOperationSet
	}
	return operation
}

func (operation ExtensionStorageOperationKind) validForInput() bool {
	spec, valid := operation.spec()
	return valid && spec.allowedInput
}

func (operation ExtensionStorageOperationKind) spec() (extensionStorageOperationSpec, bool) {
	switch operation {
	case ExtensionStorageOperationSet,
		ExtensionStorageOperationMerge,
		ExtensionStorageOperationAppend:
		return extensionStorageOperationSpec{
			allowedInput: true,
			scope:        extensionStorageOperationScopeKey,
			value:        extensionStorageValueRequired,
		}, true
	case ExtensionStorageOperationRemove:
		return extensionStorageOperationSpec{
			scope: extensionStorageOperationScopeKey,
			value: extensionStorageValueForbidden,
		}, true
	case ExtensionStorageOperationClear:
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

func mutateStorageValue(
	document any,
	path string,
	operation ExtensionStorageOperationKind,
	value any,
) (any, bool, error) {
	if path == "" {
		mutation, err := operation.mutate(document, value)
		if err != nil {
			return document, false, err
		}
		return mutation.value, mutation.changed && !mutation.remove, nil
	}

	if document == nil {
		document = map[string]any{}
	}
	return mutateNestedStorageValue(document, path, operation, value)
}

func mutateNestedStorageValue(
	document any,
	path string,
	operation ExtensionStorageOperationKind,
	value any,
) (any, bool, error) {
	container := gabs.Wrap(document)
	exists := container.ExistsP(path)
	var current any
	if exists {
		current = container.Path(path).Data()
	}
	mutation, err := operation.mutate(current, value)
	if err != nil {
		return document, false, err
	}
	if mutation.remove {
		if !exists {
			return document, false, nil
		}
		if err := container.DeleteP(path); err != nil {
			return document, false, err
		}
		return container.Data(), true, nil
	}
	if _, err := container.SetP(mutation.value, path); err != nil {
		return document, false, err
	}
	return container.Data(), mutation.changed, nil
}

func (operation ExtensionStorageOperationKind) mutate(
	current,
	value any,
) (storageValueMutation, error) {
	switch operation {
	case ExtensionStorageOperationSet:
		return storageValueMutation{value: value, changed: true}, nil
	case ExtensionStorageOperationMerge:
		merged, err := mergeJSONObjects(current, value)
		return storageValueMutation{value: merged, changed: err == nil}, err
	case ExtensionStorageOperationAppend:
		appended, err := appendUniqueJSON(current, value)
		return storageValueMutation{value: appended, changed: err == nil}, err
	case ExtensionStorageOperationRemove:
		return storageValueMutation{value: current, changed: true, remove: true}, nil
	default:
		return storageValueMutation{value: current}, fmt.Errorf(
			"unsupported operation %q",
			operation,
		)
	}
}

func mergeJSONObjects(current, additions any) (map[string]any, error) {
	additionMap, ok := additions.(map[string]any)
	if !ok {
		return nil, errors.New("merge value must be an object")
	}
	if current == nil {
		current = map[string]any{}
	}
	currentMap, ok := current.(map[string]any)
	if !ok {
		return nil, errors.New("existing value must be an object for merge")
	}
	for key, addition := range additionMap {
		additionObject, additionIsObject := addition.(map[string]any)
		currentObject, currentIsObject := currentMap[key].(map[string]any)
		if additionIsObject && currentIsObject {
			merged, err := mergeJSONObjects(currentObject, additionObject)
			if err != nil {
				return nil, err
			}
			currentMap[key] = merged
			continue
		}
		currentMap[key] = addition
	}
	return currentMap, nil
}

func appendUniqueJSON(current, additions any) ([]any, error) {
	additionList, ok := additions.([]any)
	if !ok {
		return nil, errors.New("append value must be an array")
	}
	if current == nil {
		current = []any{}
	}
	currentList, ok := current.([]any)
	if !ok {
		return nil, errors.New("existing value must be an array for append")
	}
	for _, addition := range additionList {
		if !slices.ContainsFunc(currentList, func(existing any) bool {
			return reflect.DeepEqual(existing, addition)
		}) {
			currentList = append(currentList, addition)
		}
	}
	return currentList, nil
}

func (area ExtensionStorageArea) directory() (string, error) {
	switch area {
	case ExtensionStorageAreaLocal:
		return localExtensionSettingsDir, nil
	case ExtensionStorageAreaSync:
		return syncExtensionSettingsDir, nil
	default:
		return "", fmt.Errorf("unsupported storage area %q", area)
	}
}

func rawJSONValue(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var value any
	if err := decodeJSON(bytes.NewReader(raw), &value); err != nil {
		return nil, err
	}
	return value, nil
}

func decodeStorageValue(raw []byte, encoding ExtensionStorageEncoding) (any, error) {
	if encoding == ExtensionStorageEncodingLZStringURI {
		var compressed string
		if err := json.Unmarshal(raw, &compressed); err != nil {
			return nil, err
		}
		decoded, err := lzstring.DecompressFromEncodedURIComponent(compressed)
		if err != nil {
			return nil, err
		}
		raw = []byte(decoded)
	} else if encoding != ExtensionStorageEncodingJSON {
		return nil, fmt.Errorf("unsupported encoding %q", encoding)
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var document any
	if err := decodeJSON(bytes.NewReader(raw), &document); err != nil {
		return nil, err
	}
	return document, nil
}

func encodeStorageValue(document any, encoding ExtensionStorageEncoding) ([]byte, error) {
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	if encoding == ExtensionStorageEncodingJSON {
		return encoded, nil
	}
	if encoding != ExtensionStorageEncodingLZStringURI {
		return nil, fmt.Errorf("unsupported encoding %q", encoding)
	}
	compressed, err := lzstring.CompressToEncodedURIComponent(string(encoded))
	if err != nil {
		return nil, err
	}
	return json.Marshal(compressed)
}

func validateSyncStorageState(state map[string][]byte) error {
	itemCount := 0
	totalBytes := 0
	for _, key := range slices.Sorted(maps.Keys(state)) {
		value := state[key]
		itemCount++
		size := len(key) + len(value)
		if size > syncStorageQuotaBytesPerItem {
			return fmt.Errorf(
				"sync item %q uses %d bytes, exceeding the %d-byte limit",
				key,
				size,
				syncStorageQuotaBytesPerItem,
			)
		}
		totalBytes += size
	}
	if itemCount > syncStorageMaxItems {
		return fmt.Errorf(
			"sync storage contains %d items, exceeding the %d-item limit",
			itemCount,
			syncStorageMaxItems,
		)
	}
	if totalBytes > syncStorageQuotaBytes {
		return fmt.Errorf(
			"sync storage uses %d bytes, exceeding the %d-byte limit",
			totalBytes,
			syncStorageQuotaBytes,
		)
	}
	return nil
}

func withStorage(
	profileDir,
	area,
	extensionID string,
	operation func(*leveldb.DB) error,
) (err error) {
	path := filepath.Join(profileDir, area, extensionID)
	if err := os.MkdirAll(path, fileutil.DefaultDirPerm); err != nil {
		return fmt.Errorf("create storage directory %s: %w", path, err)
	}
	database, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return fmt.Errorf("open storage %s: %w", path, err)
	}
	defer func() { err = errors.Join(err, database.Close()) }()
	return operation(database)
}

func isStorageTemporarilyUnavailable(err error) bool {
	if errors.Is(err, storage.ErrLocked) || errors.Is(err, syscall.EAGAIN) {
		return true
	}
	corrupted, ok := errors.AsType[*leveldberrors.ErrCorrupted](err)
	if !ok {
		return false
	}
	_, ok = errors.AsType[*leveldberrors.ErrMissingFiles](corrupted.Err)
	return ok
}
