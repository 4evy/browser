package extensions

import (
	"strings"
	"testing"
)

func TestValidateCatalogReportsAllInvalidFields(t *testing.T) {
	negative := -1
	err := ValidateCatalog(Catalog{
		Network: NetworkConfig{
			ChromeVersion: "latest",
			UserAgent:     "invalid\x7f",
			RetryMax:      &negative,
			Headers: map[string]string{
				"Bad Header":  "value",
				"X-Bad-Value": "value\x7f",
			},
		},
		UpdateURL: []UpdateURLExtension{{
			Name:      "Broken update",
			ID:        "invalid",
			UpdateURL: "not a URL",
		}},
		CRX: []DownloadedExtension{{
			Name:    "Broken download",
			ID:      "invalid",
			Version: "latest",
			URL:     "not a URL",
			SHA256:  "invalid",
		}},
		ZIP: []GitHubReleaseExtension{{
			ID:           "invalid",
			Name:         "Broken pinned ZIP",
			UpdatePolicy: "pinned",
			Version:      "latest",
			URL:          "not a URL",
			SHA256:       "invalid",
		}},
	})
	if err == nil {
		t.Fatal("expected invalid catalog")
	}
	for _, message := range []string{
		"extensions.network.chrome_version is invalid",
		"extensions.network.user_agent contains an invalid control character",
		"extensions.network.retry_max must not be negative",
		`extensions.network header name "Bad Header" is invalid`,
		`extensions.network header "X-Bad-Value" has an invalid value`,
		`update URL extension "Broken update" has an invalid ID`,
		`update URL extension "Broken update" has an invalid update URL`,
		`downloaded extension "Broken download" has an invalid ID`,
		`downloaded extension "Broken download" has an invalid version`,
		`downloaded extension "Broken download" has an invalid URL`,
		`downloaded extension "Broken download" has an invalid SHA-256`,
		`ZIP extension "Broken pinned ZIP" has an invalid ID`,
		`pinned ZIP extension "Broken pinned ZIP" has an invalid version`,
		`pinned ZIP extension "Broken pinned ZIP" has an invalid URL`,
		`pinned ZIP extension "Broken pinned ZIP" has an invalid SHA-256`,
	} {
		if !strings.Contains(err.Error(), message) {
			t.Errorf("validation error %q does not contain %q", err, message)
		}
	}
}

func TestValidExternalVersionUsesChromeVersionRules(t *testing.T) {
	for _, version := range []string{"1", "0.1", "1.2", "1.2.3", "1.2.3.65535"} {
		if !ValidExternalVersion(version) {
			t.Errorf("ValidExternalVersion(%q) = false, want true", version)
		}
	}
	for _, version := range []string{
		"",
		"0",
		"0.0.0.0",
		"01.2",
		"1.",
		"1.2.3.4.5",
		"1.2.3.65536",
		"1.2.beta",
		"v1.2.3",
	} {
		if ValidExternalVersion(version) {
			t.Errorf("ValidExternalVersion(%q) = true, want false", version)
		}
	}
}
