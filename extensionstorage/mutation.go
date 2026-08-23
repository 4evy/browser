package extensionstorage

import (
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/Jeffail/gabs/v2"
)

func mutateStorageValue(
	document any,
	path string,
	operation OperationKind,
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
	operation OperationKind,
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

func (operation OperationKind) mutate(
	current,
	value any,
) (storageValueMutation, error) {
	switch operation {
	case OperationSet:
		return storageValueMutation{value: value, changed: true}, nil
	case OperationMerge:
		merged, err := mergeJSONObjects(current, value)
		return storageValueMutation{value: merged, changed: err == nil}, err
	case OperationAppend:
		appended, err := appendUniqueJSON(current, value)
		return storageValueMutation{value: appended, changed: err == nil}, err
	case OperationRemove:
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
	document := gabs.Wrap(currentMap)
	if err := document.MergeFn(
		gabs.Wrap(additionMap),
		func(_, source any) any { return source },
	); err != nil {
		return nil, err
	}
	return document.Data().(map[string]any), nil
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
