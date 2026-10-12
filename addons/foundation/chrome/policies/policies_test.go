package policies

import (
	json "encoding/json"
	"reflect"
	"testing"
)

func TestResolvePresenceConflictsAndIndependentValues(t *testing.T) {
	catalog := MustLoad([]byte(`{"audited_against":"test","entries":{"sync":{"policy":"SyncDisabled","type":"bool","inverse":true,"source":"test"},"mode":{"policy":"Mode","type":"choice","choices":{"off":0,"on":1},"source":"test"}}}`))
	config := struct {
		Sync *bool  `toml:"sync"`
		Mode string `toml:"mode"`
	}{Sync: new(false), Mode: "off"}
	raw := map[string]any{"Mode": 0, "Nested": map[string]any{"integer": int64(9007199254740993)}}
	values, err := catalog.Resolve(&config, raw)
	if err != nil {
		t.Fatal(err)
	}
	if values["SyncDisabled"] != true || values["Mode"] != json.Number("0") || values["Nested"].(map[string]any)["integer"] != json.Number("9007199254740993") {
		t.Fatalf("explicit values or integer precision lost: %#v", values)
	}
	values["Nested"].(map[string]any)["integer"] = 1
	if raw["Nested"].(map[string]any)["integer"] != int64(9007199254740993) {
		t.Fatal("resolved policy values alias input")
	}
	if _, err := catalog.Resolve(config, map[string]any{"Mode": 1}); err == nil {
		t.Fatal("accepted disagreeing raw policy")
	}
	config.Sync, config.Mode = nil, ""
	values, err = catalog.Resolve(config, nil)
	if err != nil || len(values) != 0 {
		t.Fatalf("omitted values create policies: %#v, %v", values, err)
	}
}

func TestResolveRejectsIncompatibleConfigs(t *testing.T) {
	catalog := Catalog{Entries: map[string]Rule{"value": {Type: "bool", Policy: "Value"}}}
	for _, config := range []any{nil, (*struct{})(nil), 1, map[string]any{}, struct {
		Value string `toml:"value"`
	}{Value: "wrong"}} {
		if _, err := catalog.Resolve(config, nil); err == nil {
			t.Errorf("accepted invalid config type %v", reflect.TypeOf(config))
		}
	}
}
