package zen

import (
	"bytes"
	"encoding/binary"
	jsonv1 "encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/4evy/tweezle/addons/firefox"
)

func TestSessionCompression(t *testing.T) {
	for _, data := range [][]byte{nil, []byte(`{"spaces":[]}`), bytes.Repeat([]byte("workspace"), 1024), []byte{0, 1, 2, 3, 255, 254}} {
		encoded, err := encodeSession(data)
		if err != nil {
			t.Fatal(err)
		}
		got, err := decodeSession(encoded)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("round trip %d bytes: %v", len(data), err)
		}
	}
	oversized := append(bytes.Clone(sessionHeader), make([]byte, 4)...)
	binary.LittleEndian.PutUint32(oversized[8:], maximumSessionSize+1)
	for _, encoded := range [][]byte{nil, []byte("not Mozilla LZ4"), oversized, append(bytes.Clone(sessionHeader), 1, 0, 0, 0, 0xff)} {
		if _, err := decodeSession(encoded); err == nil {
			t.Fatalf("accepted corrupt session %x", encoded)
		}
	}
}

func FuzzSessionCompression(f *testing.F) {
	f.Add([]byte(`{"spaces":[],"tabs":[]}`))
	f.Add([]byte{0, 255, 1, 254})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		encoded, err := encodeSession(data)
		if err != nil {
			t.Fatal(err)
		}
		got, err := decodeSession(encoded)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("session round trip: %v", err)
		}
	})
}

func TestWorkspacePatchPreservesOtherSessionData(t *testing.T) {
	const first = "00000000-0000-0000-0000-000000000001"
	const second = "00000000-0000-0000-0000-000000000002"
	directory := t.TempDir()
	current := []byte(`{"unknown":9007199254740993,"tabs":[{"id":"keep"}],"spaces":[{"uuid":"` + first + `","name":"First","unknown":true},{"uuid":"` + second + `","name":"Second"}]}`)
	encoded, err := encodeSession(current)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "zen-sessions.jsonlz4")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	config := Config{Workspaces: map[string]map[string]any{first: {"name": "Renamed"}}, WorkspaceOrder: []string{second}}
	file, err := config.workspaces(firefox.Profile{Directory: directory})
	if err != nil {
		t.Fatal(err)
	}
	data, err := decodeSession(file.Data)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Unknown jsonv1.Number
		Tabs    []map[string]any
		Spaces  []map[string]any
	}
	if err := jsonv1.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Unknown != "9007199254740993" || got.Tabs[0]["id"] != "keep" || got.Spaces[0]["uuid"] != second || got.Spaces[1]["name"] != "Renamed" || got.Spaces[1]["unknown"] != true {
		t.Fatalf("workspace patch lost native session data: %s", data)
	}
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(unchanged, encoded) {
		t.Fatal("preparation wrote to the session")
	}
}

func TestWorkspacePatchRejectsMalformedNativeIDs(t *testing.T) {
	const id = "00000000-0000-0000-0000-000000000001"
	for _, spaces := range []string{
		`[{"name":"missing UUID"}]`,
		`[{"uuid":123,"name":"numeric UUID"}]`,
		`[{"uuid":"invalid","name":"invalid UUID"}]`,
		`[{"uuid":"` + id + `"},{"uuid":"` + id + `"}]`,
	} {
		directory := t.TempDir()
		path := filepath.Join(directory, "zen-sessions.jsonlz4")
		encoded, err := encodeSession([]byte(`{"spaces":` + spaces + `,"tabs":[{"id":"keep"}]}`))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, encoded, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := (Config{WorkspaceOrder: []string{id}}).workspaces(firefox.Profile{Directory: directory}); err == nil {
			t.Errorf("accepted malformed native spaces %s", spaces)
		}
		unchanged, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(unchanged, encoded) {
			t.Fatal("changed the native session after a preparation error")
		}
	}
}
