package chromium

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	fileutil "github.com/4evy/tweezle/internal/file"
)

type extensionStorageStage struct {
	root         string
	paths        []string
	fingerprints [][32]byte
	keep         bool
}

func stageExtensionStorage(profile, id string) (*extensionStorageStage, error) {
	if err := os.MkdirAll(profile, fileutil.PrivateDirPerm); err != nil {
		return nil, err
	}
	// Keep renames on the profile's filesystem, including symlinked profiles
	root, err := os.MkdirTemp(profile, ".tweezle-runtime-")
	if err != nil {
		return nil, err
	}
	origin := "chrome-extension_" + id + "_0.indexeddb"
	stage := &extensionStorageStage{
		root: root,
		paths: []string{
			filepath.Join("Local Extension Settings", id),
			filepath.Join("Sync Extension Settings", id),
			filepath.Join("IndexedDB", origin+".leveldb"),
			filepath.Join("IndexedDB", origin+".blob"),
		},
	}
	for _, relative := range stage.paths {
		source := filepath.Join(profile, relative)
		before, err := fingerprintStorage(source)
		if err == nil && before != ([32]byte{}) {
			target := filepath.Join(root, "Default", relative)
			err = os.MkdirAll(target, fileutil.PrivateDirPerm)
			if err == nil {
				err = os.CopyFS(target, os.DirFS(source))
			}
		}
		if err == nil {
			var after [32]byte
			after, err = fingerprintStorage(source)
			if err == nil && after != before {
				err = errors.New("extension storage changed while staging; close the browser and retry")
			}
		}
		if err != nil {
			stage.remove()
			return nil, err
		}
		stage.fingerprints = append(stage.fingerprints, before)
	}
	return stage, nil
}

func (stage *extensionStorageStage) remove() {
	if !stage.keep {
		removeBrowserProbeProfile(stage.root)
	}
}

func fingerprintStorage(directory string) ([32]byte, error) {
	var digest [32]byte
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return digest, nil
	}
	if err != nil {
		return digest, err
	}
	if !info.IsDir() {
		return digest, fmt.Errorf("storage path is not a directory: %s", directory)
	}
	hash := sha256.New()
	err = fs.WalkDir(os.DirFS(directory), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			_, _ = fmt.Fprintf(hash, "dir:%s\x00", name)
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported storage entry: %s", filepath.Join(directory, name))
		}
		_, _ = fmt.Fprintf(hash, "file:%s:%d\x00", name, info.Size())
		file, err := os.Open(filepath.Join(directory, name))
		if err != nil {
			return err
		}
		_, err = io.Copy(hash, file)
		return errors.Join(err, file.Close())
	})
	copy(digest[:], hash.Sum(nil))
	return digest, err
}

func (stage *extensionStorageStage) commit(profile string) error {
	if err := EnsureProfileNotRunning(profile); err != nil {
		return err
	}
	for i, relative := range stage.paths {
		current, err := fingerprintStorage(filepath.Join(profile, relative))
		if err != nil {
			return err
		}
		if current != stage.fingerprints[i] {
			return errors.New("extension storage changed during configuration; close the browser and retry")
		}
	}
	type replacement struct {
		target, backup      string
		previous, installed bool
	}
	var replacements []replacement
	rollback := func(cause error) error {
		for _, item := range slices.Backward(replacements) {
			if item.installed {
				if err := os.RemoveAll(item.target); err != nil {
					stage.keep = true
					cause = errors.Join(cause, err)
					continue
				}
			}
			if item.previous {
				if err := os.Rename(item.backup, item.target); err != nil {
					stage.keep = true
					cause = errors.Join(cause, err)
				}
			}
		}
		if stage.keep {
			cause = errors.Join(cause, fmt.Errorf("previous storage retained for recovery at %s", stage.root))
		}
		return cause
	}
	for i, relative := range stage.paths {
		target := filepath.Join(profile, relative)
		source := filepath.Join(stage.root, "Default", relative)
		if _, err := fingerprintStorage(source); err != nil {
			return rollback(err)
		}
		if err := os.MkdirAll(filepath.Dir(target), fileutil.PrivateDirPerm); err != nil {
			return rollback(err)
		}
		backup := filepath.Join(stage.root, fmt.Sprintf("previous-%d", i))
		replacements = append(replacements, replacement{target: target, backup: backup})
		item := &replacements[len(replacements)-1]
		if err := os.Rename(target, backup); err == nil {
			item.previous = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return rollback(err)
		}
		if err := os.Rename(source, target); err == nil {
			item.installed = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return rollback(err)
		}
	}
	return nil
}
