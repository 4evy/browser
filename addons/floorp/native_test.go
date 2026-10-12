package floorp

import (
	"fmt"
	"testing"

	"github.com/4evy/tweezle/addons/firefox"
)

func TestPrepareRejectsUnsupportedNativeShortcutStore(t *testing.T) {
	config := Config{Keyboard: map[string]any{"enabled": false}}
	for _, current := range []string{
		`{"schemaVersion":3,"shortcuts":{}}`,
		`{"schemaVersion":2,"shortcuts":{"close":{"key":1,"action":"close","modifiers":{"alt":false,"ctrl":false,"meta":false,"shift":false}}}}`,
		`{"schemaVersion":2,"shortcuts":{"close":{"key":"w","action":"close","modifiers":{"alt":"false","ctrl":false,"meta":false,"shift":false}}}}`,
	} {
		profile := firefox.Profile{Preferences: map[string]any{"floorp.keyboardshortcut.config": current}}
		if _, err := config.Prepare(profile); err == nil {
			t.Fatalf("accepted incompatible shortcut store %s", current)
		}
	}
}

func TestNativeWorkspaceReferences(t *testing.T) {
	const id = "00000000-0000-0000-0000-000000000001"
	config := Config{WorkspaceData: map[string]any{"data": map[string]any{id: map[string]any{"name": "Renamed"}}}}
	valid := fmt.Sprintf(`{"defaultID":%q,"data":{%q:{"name":"Old","userContextId":0,"future":true}},"order":[%q],"future":true}`, id, id, id)
	if _, err := config.Prepare(firefox.Profile{Preferences: map[string]any{"floorp.workspaces.v4.store": valid}}); err != nil {
		t.Fatal(err)
	}
	for _, current := range []string{
		fmt.Sprintf(`{"defaultID":%q,"data":{%q:{"name":"Old","userContextId":0}},"order":[%q,%q]}`, id, id, id, id),
		fmt.Sprintf(`{"defaultID":%q,"data":{%q:{"name":"Old","userContextId":0}},"order":[]}`, id, id),
		fmt.Sprintf(`{"defaultID":"00000000-0000-0000-0000-000000000002","data":{%q:{"name":"Old","userContextId":0}},"order":[%q]}`, id, id),
		fmt.Sprintf(`{"defaultID":%q,"data":{%q:{"name":"Old","userContextId":"wrong"}},"order":[%q]}`, id, id, id),
	} {
		if _, err := config.Prepare(firefox.Profile{Preferences: map[string]any{"floorp.workspaces.v4.store": current}}); err == nil {
			t.Fatalf("accepted incompatible workspace store %s", current)
		}
	}
}

func TestPreparePreservesFutureNativeShortcutFields(t *testing.T) {
	config := Config{Keyboard: map[string]any{"enabled": false}}
	profile := firefox.Profile{Preferences: map[string]any{
		"floorp.keyboardshortcut.config": `{"schemaVersion":2,"future":true,"shortcuts":{"close":{"key":"w","action":"close","future":true,"modifiers":{"alt":false,"ctrl":true,"meta":false,"shift":false,"future":true}}}}`,
	}}
	contribution, err := config.Prepare(profile)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"enabled":false,"future":true,"schemaVersion":2,"shortcuts":{"close":{"action":"close","future":true,"key":"w","modifiers":{"alt":false,"ctrl":true,"future":true,"meta":false,"shift":false}}}}`
	if got := contribution.Preferences["floorp.keyboardshortcut.config"]; got != want {
		t.Fatalf("unexpected native merge: %v", got)
	}
}
