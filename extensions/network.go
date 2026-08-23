package extensions

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/4evy/browser/internal/fileutil"
	"github.com/carlmjohnson/requests"
	"github.com/google/go-github/v90/github"
	"github.com/hashicorp/go-retryablehttp"
)

const (
	defaultHTTPTimeout      = time.Minute
	defaultHTTPRetryWaitMin = 200 * time.Millisecond
	defaultHTTPRetryWaitMax = 2 * time.Second
	defaultHTTPRetryMax     = 3
	latestChromeVersionURL  = "https://googlechromelabs.github.io/chrome-for-testing/LATEST_RELEASE_STABLE"
	defaultUserAgent        = "github.com/4evy/browser"
	userAgentPrefix         = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/"
	userAgentSuffix         = " Safari/537.36"
	githubAPIURL            = "https://api.github.com/"
)

func requireSuccessfulHTTPStatus(response *http.Response) error {
	if response.StatusCode >= http.StatusOK &&
		response.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	return fmt.Errorf(
		"%w: unexpected status: %d",
		(*requests.ResponseError)(response),
		response.StatusCode,
	)
}

type HTTPClient struct {
	Client        *http.Client
	GitHubToken   string
	UserAgent     string
	ChromeVersion string
	Headers       map[string]string
	Timeout       time.Duration
	RetryMax      *int
	RetryWaitMin  time.Duration
	RetryWaitMax  time.Duration
}

func (client HTTPClient) Download(ctx context.Context, target, rawURL string) (err error) {
	return fileutil.WriteWith(
		target,
		fileutil.DefaultFilePerm,
		func(writer io.Writer) error {
			return client.request(rawURL).
				ToWriter(writer).
				Fetch(ctx)
		},
	)
}

func (client HTTPClient) ResolveURL(ctx context.Context, rawURL string) (string, error) {
	var resolvedURL string
	err := client.request(rawURL).
		Head().
		Handle(func(response *http.Response) error {
			resolvedURL = response.Request.URL.String()
			return nil
		}).
		Fetch(ctx)
	if err != nil {
		return "", err
	}
	return resolvedURL, nil
}

func (client HTTPClient) ResolveLatestChromeVersion(ctx context.Context) (string, error) {
	var version string
	err := client.request(latestChromeVersionURL).
		ToString(&version).
		Fetch(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch latest stable Chrome version: %w", err)
	}
	version = strings.TrimSpace(version)
	if !ValidExternalVersion(version) {
		return "", fmt.Errorf("latest stable Chrome version is invalid: %q", version)
	}
	return version, nil
}

func (client HTTPClient) ResolveLatestGitHubRelease(
	ctx context.Context,
	repository,
	assetTemplate string,
) (ReleaseArtifact, error) {
	if !ValidGitHubRepository(repository) {
		return ReleaseArtifact{}, fmt.Errorf("invalid GitHub repository %q", repository)
	}
	owner, name, _ := strings.Cut(repository, "/")
	options := []github.ClientOptionsFunc{
		github.WithHTTPClient(client.httpClient(githubAPIURL)),
		github.WithUserAgent(client.userAgent()),
	}
	if client.GitHubToken != "" {
		options = append(options, github.WithAuthToken(client.GitHubToken))
	}
	api, err := github.NewClient(options...)
	if err != nil {
		return ReleaseArtifact{}, fmt.Errorf("create GitHub client: %w", err)
	}
	release, _, err := api.Repositories.GetLatestRelease(ctx, owner, name)
	if err != nil {
		return ReleaseArtifact{}, fmt.Errorf("get latest GitHub release for %s: %w", repository, err)
	}
	tag := release.GetTagName()
	if tag == "" {
		return ReleaseArtifact{}, fmt.Errorf("latest GitHub release for %s has no tag", repository)
	}
	assetName := strings.ReplaceAll(assetTemplate, releaseTagPlaceholder, tag)
	for _, asset := range release.Assets {
		if asset.GetName() != assetName {
			continue
		}
		checksum, err := NormalizeSHA256(asset.GetDigest())
		if err != nil {
			return ReleaseArtifact{}, fmt.Errorf("asset %s has no usable GitHub digest: %w", assetName, err)
		}
		version, _ := strings.CutPrefix(tag, releaseVersionPrefix)
		return ReleaseArtifact{
			Version: version,
			URL:     asset.GetBrowserDownloadURL(),
			SHA256:  checksum,
		}, nil
	}
	return ReleaseArtifact{}, fmt.Errorf("release %s has no asset named %s", tag, assetName)
}

func (client HTTPClient) request(rawURL string) *requests.Builder {
	return requests.URL(rawURL).
		Client(client.httpClient(rawURL)).
		UserAgent(client.userAgent()).
		AddValidator(requireSuccessfulHTTPStatus)
}

func (client HTTPClient) httpClient(headerURL string) *http.Client {
	base := client.Client
	if base == nil {
		base = &http.Client{Timeout: cmp.Or(client.Timeout, defaultHTTPTimeout)}
	}
	origin := requestOrigin(headerURL)
	if len(client.Headers) > 0 && origin != "" {
		copy := *base
		transport := base.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		copy.Transport = headerTransport{
			Base:    transport,
			Headers: client.Headers,
			Origin:  origin,
		}
		base = &copy
	}
	retry := retryablehttp.NewClient()
	retry.HTTPClient = base
	retry.Logger = nil
	retry.RetryWaitMin = cmp.Or(client.RetryWaitMin, defaultHTTPRetryWaitMin)
	retry.RetryWaitMax = cmp.Or(client.RetryWaitMax, defaultHTTPRetryWaitMax)
	retry.RetryMax = defaultHTTPRetryMax
	if client.RetryMax != nil {
		retry.RetryMax = *client.RetryMax
	}
	return retry.StandardClient()
}

type headerTransport struct {
	Base    http.RoundTripper
	Headers map[string]string
	Origin  string
}

func (transport headerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	if requestOrigin(request.URL.String()) == transport.Origin {
		for name, value := range transport.Headers {
			request.Header.Set(name, value)
		}
	}
	return transport.Base.RoundTrip(request)
}

func requestOrigin(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func (client HTTPClient) userAgent() string {
	if client.UserAgent != "" {
		return client.UserAgent
	}
	if client.ChromeVersion != "" {
		return chromeUserAgent(client.ChromeVersion)
	}
	return defaultUserAgent
}

func (config NetworkConfig) HTTPClient(githubToken string) HTTPClient {
	return HTTPClient{
		GitHubToken:   githubToken,
		UserAgent:     config.UserAgent,
		ChromeVersion: config.ChromeVersion,
		Headers:       config.Headers,
		Timeout:       time.Duration(config.TimeoutSeconds) * time.Second,
		RetryMax:      config.RetryMax,
		RetryWaitMin:  time.Duration(config.RetryWaitMinMilliseconds) * time.Millisecond,
		RetryWaitMax:  time.Duration(config.RetryWaitMaxMilliseconds) * time.Millisecond,
	}
}

func chromeUserAgent(version string) string {
	return userAgentPrefix + version + userAgentSuffix
}
