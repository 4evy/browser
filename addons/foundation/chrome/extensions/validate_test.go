package extensions

import (
	"strings"
	"testing"
)

func TestValidateIDAliasesReportsEveryInvalidID(t *testing.T) {
	err := ValidateIDAliases(map[string]string{
		"invalid source":                   "also-invalid",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": "../outside",
	})
	if err == nil {
		t.Fatal("invalid aliases were accepted")
	}
	for _, message := range []string{
		`invalid extension ID alias source "invalid source"`,
		`invalid installed extension ID "also-invalid" for alias "invalid source"`,
		`invalid installed extension ID "../outside" for alias "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`,
	} {
		if !strings.Contains(err.Error(), message) {
			t.Errorf("validation error %q does not contain %q", err, message)
		}
	}
}

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
		Git: []GitExtension{{
			ID:           "invalid",
			Name:         "Broken Git",
			Provider:     GitProviderGit,
			Repository:   "must/not/be/set",
			URL:          "not a Git URL",
			UpdatePolicy: UpdatePolicyPinned,
			Ref:          "bad ref",
			Commit:       "short",
			Subdirectory: "../outside",
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
		`Git extension "Broken Git" has an invalid ID`,
		`git extension "Broken Git" with provider "git" must not set repository`,
		`git extension "Broken Git" has an invalid Git URL`,
		`git extension "Broken Git" has an invalid ref`,
		`pinned Git extension "Broken Git" has an invalid commit`,
		`git extension "Broken Git" has an invalid subdirectory`,
	} {
		if !strings.Contains(err.Error(), message) {
			t.Errorf("validation error %q does not contain %q", err, message)
		}
	}
}

func TestValidateCatalogAcceptsHostedAndGenericGitExtensions(t *testing.T) {
	err := ValidateCatalog(Catalog{Git: []GitExtension{
		{
			ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Name: "GitHub",
			Provider: GitProviderGitHub, Repository: "owner/project", Ref: "main",
		},
		{
			ID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Name: "GitLab",
			Provider: GitProviderGitLab, Repository: "group/team/project", Ref: "v1.2.3",
		},
		{
			ID: "cccccccccccccccccccccccccccccccc", Name: "Codeberg",
			Provider: GitProviderCodeberg, Repository: "owner/project",
		},
		{
			ID: "dddddddddddddddddddddddddddddddd", Name: "SourceHut",
			Provider: GitProviderSourceHut, Repository: "~owner/project",
		},
		{
			ID: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", Name: "Generic pinned Git",
			URL: "ssh://git@example.test/team/project.git", Ref: "main",
			Commit:       "0123456789abcdef0123456789abcdef01234567",
			Subdirectory: "dist/extension",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateCatalogRejectsDuplicateExtensionIDs(t *testing.T) {
	const duplicateID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	err := ValidateCatalog(Catalog{
		ChromeStoreUpdateURL: "https://clients2.google.com/service/update2/crx",
		ChromeStore: []ChromeStoreExtension{{
			ID: duplicateID, Name: "Store copy",
		}},
		UpdateURL: []UpdateURLExtension{{
			ID: duplicateID, Name: "Update copy", UpdateURL: "https://example.test/update",
		}},
	})
	if err == nil || !strings.Contains(
		err.Error(),
		`extension ID "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" is declared more than once `+
			`(chrome store "Store copy" and update URL "Update copy")`,
	) {
		t.Fatalf("validation error = %v, want duplicate extension ID", err)
	}
}

func TestValidExternalVersionMatchesChromiumManifestParser(t *testing.T) {
	for _, version := range []string{
		"0",
		"1",
		"0.0.0.0",
		"0.0.01",
		"1.002.3",
		"1.2.3.4294967295",
	} {
		if !ValidExternalVersion(version) {
			t.Errorf("ValidExternalVersion(%q) = false, want true", version)
		}
	}
	for _, version := range []string{
		"",
		"01.2",
		"+1.2",
		"1.",
		"1.2.3.4.5",
		"1.2.3.4294967296",
		"1.2.beta",
		"v1.2.3",
	} {
		if ValidExternalVersion(version) {
			t.Errorf("ValidExternalVersion(%q) = true, want false", version)
		}
	}
}
