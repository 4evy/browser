package browser

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func FuzzExtensionStorageSettingsParser(f *testing.F) {
	for _, seed := range [][]byte{
		{},
		[]byte(`{}`),
		[]byte(`{"schema_version":1,"local":[],"sync":[]}`),
		[]byte(`{"unknown":true}`),
		[]byte(`{"local":[{"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","values":{"nil":null,"large":9007199254740993}}]}`),
		[]byte(`{"operations":[{"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","area":"local","key":"options","operation":"merge","path":"a\\.b","value":{"enabled":true}}]}`),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var settings ExtensionStorageSettings
		if err := decodeJSONStrict(bytes.NewReader(data), &settings); err == nil {
			_ = settings.validate()
		}
	})
}

func FuzzStorageJSONRoundTrip(f *testing.F) {
	for _, seed := range []string{
		`null`,
		`""`,
		`9007199254740993`,
		`[true,false,null,{"unicode":"Здравей, 世界","escaped":"a.b"}]`,
		`{"nested":{"array":[1,1,2],"empty":{},"key.with.dots":"value"}}`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		var want any
		if err := decodeJSON(bytes.NewBufferString(input), &want); err != nil {
			t.Skip()
		}
		for _, encoding := range []ExtensionStorageEncoding{
			ExtensionStorageEncodingJSON,
			ExtensionStorageEncodingLZStringURI,
		} {
			encoded, err := encodeStorageValue(want, encoding)
			if err != nil {
				t.Fatal(err)
			}
			var got any
			got, err = decodeStorageValue(encoded, encoding)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(want, got) {
				wantJSON, _ := json.Marshal(want)
				gotJSON, _ := json.Marshal(got)
				t.Fatalf(
					"%s round trip mismatch: want %s, got %s",
					encoding,
					wantJSON,
					gotJSON,
				)
			}
		}
	})
}
