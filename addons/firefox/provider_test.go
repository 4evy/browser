package firefox

import "testing"

type nilProvider struct{}

func (*nilProvider) Variant() string                       { panic("called nil provider") }
func (*nilProvider) Validate() error                       { panic("called nil provider") }
func (*nilProvider) Prepare(Profile) (Contribution, error) { panic("called nil provider") }

func TestRejectTypedNilProvider(t *testing.T) {
	config := Config{Providers: []Provider{(*nilProvider)(nil)}}
	if err := config.Validate(); err == nil {
		t.Fatal("accepted typed nil provider")
	}
}
