package fileutil

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var errReadFailed = errors.New("read failed")

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errReadFailed
}

func TestWriteReaderAtomicallyReplacesWithRequestedPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "executable")
	if err := os.MkdirAll(filepath.Dir(path), DefaultDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old"), PrivateFilePerm); err != nil {
		t.Fatal(err)
	}

	if err := WriteReader(path, strings.NewReader("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("content = %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Fatalf("permissions = %o", got)
	}
}

func TestWriteReaderLeavesTargetUnchangedOnReadFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(path, []byte("original"), PrivateFilePerm); err != nil {
		t.Fatal(err)
	}
	reader := io.MultiReader(strings.NewReader("partial"), failingReader{})

	if err := WriteReader(path, reader, DefaultFilePerm); !errors.Is(err, errReadFailed) {
		t.Fatalf("error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Fatalf("content after failed write = %q", data)
	}
}

func TestWriteIfChangedWrappersReportWhetherTheyWrite(t *testing.T) {
	tests := []struct {
		name  string
		write func(string) (bool, error)
		want  string
	}{
		{
			name: "text",
			write: func(path string) (bool, error) {
				return WriteTextIfChanged(path, "value\n")
			},
			want: "value\n",
		},
		{
			name: "JSON",
			write: func(path string) (bool, error) {
				return WriteJSONIfChanged(path, map[string]any{"enabled": true}, PrivateFilePerm)
			},
			want: "{\n  \"enabled\": true\n}\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "nested", "output")
			changed, err := test.write(path)
			if err != nil {
				t.Fatal(err)
			}
			if !changed {
				t.Fatal("initial write was not reported as changed")
			}

			changed, err = test.write(path)
			if err != nil {
				t.Fatal(err)
			}
			if changed {
				t.Fatal("identical write was reported as changed")
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != test.want {
				t.Fatalf("content = %q, want %q", data, test.want)
			}
		})
	}
}
