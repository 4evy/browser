package fileutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/google/renameio/v2"
)

const (
	DefaultDirPerm  fs.FileMode = 0o755
	PrivateDirPerm  fs.FileMode = 0o700
	DefaultFilePerm fs.FileMode = 0o644
	PrivateFilePerm fs.FileMode = 0o600
	ExecutablePerm  fs.FileMode = 0o111
)

func WriteFile(path string, data []byte, perm fs.FileMode) error {
	return WriteReader(path, bytes.NewReader(data), perm)
}

func WriteReader(path string, reader io.Reader, perm fs.FileMode) error {
	return WriteWith(path, perm, func(writer io.Writer) error {
		_, err := io.Copy(writer, reader)
		return err
	})
}

func WriteWith(path string, perm fs.FileMode, write func(io.Writer) error) error {
	if err := os.MkdirAll(filepath.Dir(path), DefaultDirPerm); err != nil {
		return err
	}
	pending, err := renameio.NewPendingFile(path, renameio.WithStaticPermissions(perm))
	if err != nil {
		return err
	}
	defer func() { _ = pending.Cleanup() }()
	if err := write(pending); err != nil {
		return err
	}
	return pending.CloseAtomicallyReplace()
}

func WriteExecutable(path string, data []byte) error {
	return WriteFile(path, data, DefaultFilePerm|ExecutablePerm)
}

func WriteTextIfChanged(path, text string) (bool, error) {
	return writeIfChanged(path, []byte(text), DefaultFilePerm)
}

func WriteJSONIfChanged(path string, value any, perm fs.FileMode) (bool, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return false, err
	}
	data = append(data, '\n')
	return writeIfChanged(path, data, perm)
}

func writeIfChanged(path string, data []byte, perm fs.FileMode) (bool, error) {
	current, err := os.ReadFile(path)
	if err == nil && bytes.Equal(current, data) {
		return false, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, WriteFile(path, data, perm)
}

func CopyFile(source, target string) (err error) {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, input.Close()) }()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("source is not a regular file")
	}
	return WriteReader(target, input, info.Mode().Perm())
}
