package brave

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	browser "github.com/4evy/browser"
	"github.com/4evy/browser/chromium"
)

func TestMergeBraveManagedPoliciesRejectsCrossProfileConflicts(t *testing.T) {
	first := browser.Config{
		Browser: browser.BrowserConfig{},
		Providers: []chromium.Provider{Config{
			ManagedPolicies: map[string]any{"BrowserSignin": int64(0)},
		}},
	}
	second := browser.Config{
		Browser: browser.BrowserConfig{},
		Providers: []chromium.Provider{Config{
			ManagedPolicies: map[string]any{"BrowserSignin": int64(1)},
		}},
	}

	_, err := browser.MergeManagedPolicies(first, second)
	if err == nil ||
		!strings.Contains(err.Error(), `policy "BrowserSignin" conflicts`) {
		t.Fatalf("conflict error = %v", err)
	}
}

func TestEncodeAndWriteBraveManagedPolicyDeterministically(t *testing.T) {
	policies := map[string]any{
		"BraveWalletDisabled": true,
		"BrowserSignin":       int64(0),
	}
	want := "{\n  \"BraveWalletDisabled\": true,\n  \"BrowserSignin\": 0\n}\n"

	var encoded bytes.Buffer
	if err := chromium.EncodeManagedPolicy(&encoded, policies); err != nil {
		t.Fatal(err)
	}
	if encoded.String() != want {
		t.Fatalf("encoded policy = %q, want %q", encoded.String(), want)
	}

	path := filepath.Join(t.TempDir(), "managed", "browser.json")
	if err := chromium.WriteManagedPolicyFile(path, policies); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("policy file = %q, want %q", data, want)
	}
}

func TestBraveConfigRejectsEmptyManagedPolicyName(t *testing.T) {
	config := browser.Config{Browser: browser.BrowserConfig{
		ExecutableName: "brave",
	}, Providers: []chromium.Provider{Config{
		ManagedPolicies: map[string]any{" ": true},
	}}}

	err := config.Validate()
	if err == nil ||
		!strings.Contains(
			err.Error(),
			"managed_policies contains an empty name",
		) {
		t.Fatalf("validation error = %v", err)
	}
}
