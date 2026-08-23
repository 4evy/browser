package extensionstorage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/4evy/browser/extensions"
	"github.com/syndtr/goleveldb/leveldb"
)

// Apply validates, plans, and commits extension-storage settings in source
// order. It edits Chromium's LevelDB ValueStore representation directly, so
// the browser must be closed. Direct writes do not emit chrome.storage change
// events. Sync-area writes update the local cache only; when cloud sync already
// has state, Chromium's startup merge treats that state as authoritative and
// may overwrite these bytes.
// Source: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/chrome/browser/extensions/api/storage/syncable_settings_storage.cc#148
func Apply(ctx context.Context, options ApplyOptions) error {
	if options.ProfileDir == "" {
		return errors.New("profile directory is required")
	}
	if err := extensions.ValidateIDAliases(options.ExtensionIDAliases); err != nil {
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
	input Input,
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
					Encoding:  EncodingJSON,
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
	input Input,
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
	area Area,
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
		if plan.Area == AreaSync {
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
	case OperationClear:
		clear(state)
		return true
	case OperationRemove:
		if mutation.Path == "" {
			delete(state, mutation.Key)
			return true
		}
	case OperationSet,
		OperationMerge,
		OperationAppend:
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
	// Chromium's LeveldbValueStore also maps each storage key directly to JSON
	// bytes. One batch keeps this target's deletes and puts atomic, but bypasses
	// StorageFrontend observers and SyncableSettingsStorage by design.
	// Source: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/components/value_store/leveldb_value_store.cc#268
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
