// Package storage exposes the shared implementation in addons/foundation
package storage

import (
	"context"

	foundation "github.com/4evy/tweezle/addons/foundation/chrome/storage"
)

func Apply(ctx context.Context, options ApplyOptions) error {
	return foundation.Apply(ctx, options)
}

func IsTemporarilyUnavailable(err error) bool {
	return foundation.IsTemporarilyUnavailable(err)
}

func ValidateFiles(paths []string) error {
	return foundation.ValidateFiles(paths)
}

type Area = foundation.Area

const AreaLocal = foundation.AreaLocal

const AreaSync = foundation.AreaSync

type Encoding = foundation.Encoding

const EncodingJSON = foundation.EncodingJSON

const EncodingLZStringURI = foundation.EncodingLZStringURI

type OperationKind = foundation.OperationKind

const OperationSet = foundation.OperationSet

const OperationMerge = foundation.OperationMerge

const OperationAppend = foundation.OperationAppend

const OperationRemove = foundation.OperationRemove

const OperationClear = foundation.OperationClear

type SettingsSource = foundation.SettingsSource

type ApplyOptions = foundation.ApplyOptions

type Input = foundation.Input

type Settings = foundation.Settings

type Entry = foundation.Entry

type Operation = foundation.Operation

type InputBinding = foundation.InputBinding
