package tweezle

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"io"
	"reflect"
	"testing"
)

type testModule struct {
	validation error
	accepted   bool
	failure    error
	onRun      func()
}

func (module *testModule) Validate() error { return module.validation }
func (module *testModule) Run(context.Context, any) (bool, error) {
	if module.onRun != nil {
		module.onRun()
	}
	return module.accepted, module.failure
}

func TestDecodeRejectsUnknownSectionsBeforeCallingAddons(t *testing.T) {
	called := false
	addon := Addon{Name: "known", Sections: []string{"known"}, Decode: func([]byte, Source) (Module, error) {
		called = true
		return &testModule{}, nil
	}}
	if _, err := DecodeConfig([]byte("[known]\nvalue = true\n[unknown]\nvalue = true\n"), Source{}, addon); err == nil {
		t.Fatal("accepted unowned configuration section")
	}
	if called {
		t.Fatal("called addon before rejecting unknown sections")
	}
}

func TestRunValidatesBeforeExecutingAndStopsOnError(t *testing.T) {
	var order []string
	first := &testModule{accepted: true, onRun: func() { order = append(order, "first") }}
	second := &testModule{accepted: true, validation: errors.New("invalid"), onRun: func() { order = append(order, "second") }}
	config := Config{Modules: []Binding{{"first", first}, {"second", second}}}
	if err := config.Run(t.Context(), struct{}{}); err == nil || len(order) != 0 {
		t.Fatalf("ran modules before validation: %v, %v", order, err)
	}
	second.validation = nil
	first.failure = errors.New("operation failed")
	if err := config.Run(t.Context(), struct{}{}); err == nil || !reflect.DeepEqual(order, []string{"first"}) {
		t.Fatalf("did not stop after failure: %v, %v", order, err)
	}
	first.failure = nil
	order = nil
	if err := config.Run(t.Context(), struct{}{}); err != nil || !reflect.DeepEqual(order, []string{"first", "second"}) {
		t.Fatalf("registration order changed: %v, %v", order, err)
	}
	order = nil
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := config.Run(ctx, struct{}{}); !errors.Is(err, context.Canceled) || len(order) != 0 {
		t.Fatalf("cancelled operation ran: %v, %v", order, err)
	}
}

func TestValidateRejectsTypedNilModule(t *testing.T) {
	config := Config{Modules: []Binding{{Name: "empty", Module: (*testModule)(nil)}}}
	if err := config.Validate(); err == nil {
		t.Fatal("accepted typed nil module")
	}
}

type testRenderer struct {
	testModule
	format string
	values map[string]any
}

func (renderer *testRenderer) PolicyFormat() string { return renderer.format }
func (renderer *testRenderer) Policies() (map[string]any, error) {
	return renderer.values, nil
}
func (renderer *testRenderer) EncodePolicies(writer io.Writer, values map[string]any) error {
	return json.MarshalWrite(writer, values)
}

func TestRenderPoliciesRejectsConflictsBeforeWriting(t *testing.T) {
	first := &testRenderer{format: "chrome", values: map[string]any{"policy": false}}
	second := &testRenderer{format: "chrome", values: map[string]any{"policy": true}}
	configs := []Config{
		{Modules: []Binding{{"first", first}}},
		{Modules: []Binding{{"second", second}}},
	}
	var output bytes.Buffer
	if err := RenderPolicies(t.Context(), &output, configs...); err == nil || output.Len() != 0 {
		t.Fatalf("wrote conflicting policies: %s, %v", output.Bytes(), err)
	}
	second.values = map[string]any{"policy": false}
	second.format = "edge"
	if err := RenderPolicies(t.Context(), &output, configs...); err == nil || output.Len() != 0 {
		t.Fatalf("merged distinct policy stores: %s, %v", output.Bytes(), err)
	}
	second.format = "chrome"
	if err := RenderPolicies(t.Context(), &output, configs...); err != nil {
		t.Fatal(err)
	}
	if output.String() != `{"policy":false}` {
		t.Fatalf("unexpected merged policies: %s", output.Bytes())
	}
}
