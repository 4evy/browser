package extensionstorage

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/4evy/browser/internal/fileutil"
	"github.com/4evy/browser/internal/jsonutil"
	lzstring "github.com/daku10/go-lz-string"
	"github.com/syndtr/goleveldb/leveldb"
	leveldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

func (area Area) directory() (string, error) {
	switch area {
	case AreaLocal:
		return localExtensionSettingsDir, nil
	case AreaSync:
		return syncExtensionSettingsDir, nil
	default:
		return "", fmt.Errorf("unsupported storage area %q", area)
	}
}

func rawJSONValue(raw jsontext.Value) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	return jsonutil.Default.Decode[any](bytes.NewReader(raw))
}

func decodeStorageValue(raw []byte, encoding Encoding) (any, error) {
	if encoding == EncodingLZStringURI {
		var compressed string
		if err := json.Unmarshal(raw, &compressed); err != nil {
			return nil, err
		}
		decoded, err := lzstring.DecompressFromEncodedURIComponent(compressed)
		if err != nil {
			return nil, err
		}
		raw = []byte(decoded)
	} else if encoding != EncodingJSON {
		return nil, fmt.Errorf("unsupported encoding %q", encoding)
	}
	if len(raw) == 0 {
		return nil, nil
	}
	return jsonutil.Default.Decode[any](bytes.NewReader(raw))
}

func encodeStorageValue(document any, encoding Encoding) ([]byte, error) {
	encoded, err := json.Marshal(document, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	if encoding == EncodingJSON {
		return encoded, nil
	}
	if encoding != EncodingLZStringURI {
		return nil, fmt.Errorf("unsupported encoding %q", encoding)
	}
	compressed, err := lzstring.CompressToEncodedURIComponent(string(encoded))
	if err != nil {
		return nil, err
	}
	return json.Marshal(compressed, json.Deterministic(true))
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
	// Chromium uses one LevelDB directory per extension and storage area. Open
	// the database directly only while the browser is closed: LevelDB permits a
	// single writer and Chromium keeps these stores locked while running.
	// Source: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/extensions/browser/api/storage/value_store_util.cc#17
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

// IsTemporarilyUnavailable reports whether Chromium storage should be retried
// after the browser releases its LevelDB files.
func IsTemporarilyUnavailable(err error) bool {
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
