package chromium

import (
	"errors"
	"testing"

	"github.com/4evy/tweezle/addons/chromium/ublock"
)

type nilProvider struct{}

func (*nilProvider) Validate() error             { panic("called nil provider") }
func (*nilProvider) Contributions() Contribution { panic("called nil provider") }
func (*nilProvider) PolicyNamespace() string     { panic("called nil provider") }

func TestRejectTypedNilProvider(t *testing.T) {
	config := Config{Providers: []Provider{(*nilProvider)(nil)}}
	if err := config.Validate(); err == nil {
		t.Fatal("accepted typed nil provider")
	}
	addon := ProviderAddon{Name: "empty", Decode: func([]byte) (Provider, error) {
		return (*nilProvider)(nil), nil
	}}
	if _, err := decodeConfig([]byte("[browser.empty]\n"), []ProviderAddon{addon}); err == nil {
		t.Fatal("accepted typed nil provider from decoder")
	}
}

type invalidProvider struct{}

func (invalidProvider) Validate() error             { return errors.New("invalid provider config") }
func (invalidProvider) Contributions() Contribution { panic("used invalid provider") }

func TestValidateDoesNotUseInvalidProviderForBundledUBlock(t *testing.T) {
	config := Config{
		Browser:   BrowserConfig{ExecutableName: "test-browser"},
		Providers: []Provider{invalidProvider{}},
		UBlock:    ublock.Config{Origin: &ublock.OriginConfig{Install: new(false)}},
	}
	if err := config.Validate(); err == nil {
		t.Fatal("accepted invalid provider")
	}
}
