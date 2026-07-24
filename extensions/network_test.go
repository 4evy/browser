package extensions

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestResolveLatestGitHubReleaseUsesGitHubClient(t *testing.T) {
	const token = "test-token"
	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "request-context")
	client := HTTPClient{
		GitHubToken: token,
		Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if got := request.Context().Value(contextKey{}); got != "request-context" {
				t.Fatalf("request context value = %v", got)
			}
			if got := request.URL.String(); got != "https://api.github.com/repos/owner/repository/releases/latest" {
				t.Fatalf("request URL = %q", got)
			}
			if got := request.Header.Get("Authorization"); got != "Bearer "+token {
				t.Fatalf("authorization = %q", got)
			}
			if got := request.Header.Get("User-Agent"); got != defaultUserAgent {
				t.Fatalf("user agent = %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"tag_name": "v1.2.3",
					"assets": [{
						"name": "extension-v1.2.3.zip",
						"browser_download_url": "https://example.test/extension-v1.2.3.zip",
						"digest": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
					}]
				}`)),
			}, nil
		})},
	}

	artifact, err := client.ResolveLatestGitHubRelease(
		ctx,
		"owner/repository",
		"extension-{tag}.zip",
	)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Version != "1.2.3" {
		t.Fatalf("version = %q", artifact.Version)
	}
	if artifact.URL != "https://example.test/extension-v1.2.3.zip" {
		t.Fatalf("URL = %q", artifact.URL)
	}
	if artifact.SHA256 != "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" {
		t.Fatalf("checksum = %q", artifact.SHA256)
	}
}

func TestResolveLatestChromeVersion(t *testing.T) {
	client := HTTPClient{
		Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if got := request.URL.String(); got != latestChromeVersionURL {
				t.Fatalf("request URL = %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("152.0.7971.0\n")),
				Request:    request,
			}, nil
		})},
	}

	version, err := client.ResolveLatestChromeVersion(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if version != "152.0.7971.0" {
		t.Fatalf("version = %q", version)
	}
}

func TestResolveLatestChromeVersionRejectsInvalidResponse(t *testing.T) {
	client := HTTPClient{
		Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("<html>not a version</html>")),
				Request:    request,
			}, nil
		})},
	}

	if _, err := client.ResolveLatestChromeVersion(t.Context()); err == nil {
		t.Fatal("expected invalid latest Chrome version to fail")
	}
}

func TestDownloadHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := (HTTPClient{}).Download(
		ctx,
		t.TempDir()+"/download",
		"https://example.test/download",
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("download error = %v, want context cancellation", err)
	}
}

func TestDownloadRetriesAndAtomicallyReplacesTarget(t *testing.T) {
	attempts := 0
	client := HTTPClient{
		Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			attempts++
			if got := request.Header.Get("User-Agent"); got != defaultUserAgent {
				t.Fatalf("user agent = %q", got)
			}
			status := http.StatusOK
			body := "downloaded"
			if attempts == 1 {
				status = http.StatusServiceUnavailable
				body = "retry"
			}
			return &http.Response{
				StatusCode: status,
				Status:     http.StatusText(status),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    request,
			}, nil
		})},
	}
	target := filepath.Join(t.TempDir(), "extension.crx")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := client.Download(t.Context(), target, "https://example.test/extension.crx"); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "downloaded" {
		t.Fatalf("download = %q", got)
	}
}

func TestDownloadFailurePreservesTarget(t *testing.T) {
	client := HTTPClient{
		Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Status:     "404 Not Found",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("missing")),
				Request:    request,
			}, nil
		})},
	}
	target := filepath.Join(t.TempDir(), "extension.crx")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := client.Download(t.Context(), target, "https://example.test/missing.crx"); err == nil {
		t.Fatal("expected download to fail")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "old" {
		t.Fatalf("download failure replaced target with %q", got)
	}
}

func TestDownloadUsesConfiguredUserAgentHeadersAndRetryPolicy(t *testing.T) {
	attempts := 0
	retryMax := 0
	client := HTTPClient{
		UserAgent: "Custom Browser/7.0",
		Headers:   map[string]string{"X-Download-Token": "configured"},
		RetryMax:  &retryMax,
		Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			attempts++
			if got := request.Header.Get("User-Agent"); got != "Custom Browser/7.0" {
				t.Fatalf("user agent = %q", got)
			}
			if got := request.Header.Get("X-Download-Token"); got != "configured" {
				t.Fatalf("custom header = %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Status:     "503 Service Unavailable",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("unavailable")),
				Request:    request,
			}, nil
		})},
	}
	err := client.Download(
		t.Context(),
		filepath.Join(t.TempDir(), "download"),
		"https://example.test/download",
	)
	if err == nil {
		t.Fatal("expected configured no-retry request to fail")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want one", attempts)
	}
}

func TestNetworkConfigDerivesDownloadUserAgentFromChromeVersion(t *testing.T) {
	config := NetworkConfig{ChromeVersion: "152.0.7971.0"}
	if got := config.HTTPClient("").userAgent(); !strings.Contains(got, "Chrome/152.0.7971.0") {
		t.Fatalf("derived user agent = %q", got)
	}
	config.UserAgent = "Explicit/1.0"
	if got := config.HTTPClient("").userAgent(); got != "Explicit/1.0" {
		t.Fatalf("explicit user agent = %q", got)
	}
}
