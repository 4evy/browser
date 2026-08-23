// Package extensions downloads extension artifacts and prepares Chromium's
// external-extension and --load-extension inputs.
package extensions

import "context"

type UpdatePolicy string

type GitProvider string

const (
	UpdatePolicyLatest UpdatePolicy = "latest"
	UpdatePolicyPinned UpdatePolicy = "pinned"

	GitProviderGit       GitProvider = "git"
	GitProviderGitHub    GitProvider = "github"
	GitProviderGitLab    GitProvider = "gitlab"
	GitProviderCodeberg  GitProvider = "codeberg"
	GitProviderSourceHut GitProvider = "sourcehut"
)

type Catalog struct {
	ChromeStoreUpdateURL string                 `toml:"chrome_store_update_url"`
	Network              NetworkConfig          `toml:"network"`
	ChromeStore          []ChromeStoreExtension `toml:"chrome_store"`
	UpdateURL            []UpdateURLExtension   `toml:"update_url"`
	CRX                  []DownloadedExtension  `toml:"crx"`
	ZIP                  []ZIPExtension         `toml:"zip"`
	Git                  []GitExtension         `toml:"git"`
}

// ExtensionIdentity is shared by every catalog entry. Go 1.27 permits the
// promoted ID and Name selectors to be used directly as composite-literal
// keys, so callers keep the concise ChromeStoreExtension{ID: ..., Name: ...}
// form while the model declares these fields only once.
type ExtensionIdentity struct {
	ID   string `toml:"id"`
	Name string `toml:"name"`
}

func (identity ExtensionIdentity) catalogIdentity() (string, string) {
	return identity.ID, identity.Name
}

// NetworkConfig controls artifact resolution and downloads. ChromeVersion is
// the product version sent to a Chrome Web Store-compatible update service; it
// is not an extension manifest version.
type NetworkConfig struct {
	ChromeVersion            string            `toml:"chrome_version"`
	UserAgent                string            `toml:"user_agent"`
	Headers                  map[string]string `toml:"headers"`
	TimeoutSeconds           int               `toml:"timeout_seconds"`
	RetryMax                 *int              `toml:"retry_max"`
	RetryWaitMinMilliseconds int               `toml:"retry_wait_min_milliseconds"`
	RetryWaitMaxMilliseconds int               `toml:"retry_wait_max_milliseconds"`
}

// ChromeStoreExtension follows the newest release that the configured update
// service considers compatible with the selected Chrome product version.
type ChromeStoreExtension struct {
	ExtensionIdentity `toml:",inline"`
}

// UpdateURLExtension leaves both download and update selection to Chromium by
// writing an external_update_url definition.
type UpdateURLExtension struct {
	ExtensionIdentity `toml:",inline"`
	UpdateURL         string `toml:"update_url"`
}

// DownloadedExtension is a pinned CRX artifact. SHA256 authenticates the exact
// downloaded bytes; ID is also checked against the ID declared by the CRX3
// header before the artifact is exposed to Chromium.
type DownloadedExtension struct {
	ExtensionIdentity `toml:",inline"`
	Version           string `toml:"version"`
	URL               string `toml:"url"`
	SHA256            string `toml:"sha256"`
}

// ZIPExtension describes an unpacked extension tree delivered in a ZIP. A
// latest entry resolves a GitHub release; a pinned entry names exact version,
// URL, and digest values.
type ZIPExtension struct {
	ExtensionIdentity `toml:",inline"`
	UpdatePolicy      UpdatePolicy `toml:"update_policy"`
	Version           string       `toml:"version"`
	URL               string       `toml:"url"`
	SHA256            string       `toml:"sha256"`
	Repository        string       `toml:"repository"`
	AssetTemplate     string       `toml:"asset_template"`
	ArchiveRoot       string       `toml:"archive_root"`
	LoadUnpacked      bool         `toml:"load_unpacked"`
}

// GitHubReleaseExtension is retained as a source-compatible alias for the
// original ZIP extension type. Pinned ZIPs may come from any HTTP host.
type GitHubReleaseExtension = ZIPExtension

type GitExtension struct {
	ExtensionIdentity `toml:",inline"`
	Provider          GitProvider  `toml:"provider"`
	Repository        string       `toml:"repository"`
	URL               string       `toml:"url"`
	UpdatePolicy      UpdatePolicy `toml:"update_policy"`
	Ref               string       `toml:"ref"`
	Commit            string       `toml:"commit"`
	Subdirectory      string       `toml:"subdirectory"`
	LoadUnpacked      bool         `toml:"load_unpacked"`
}

type ReleaseArtifact struct {
	Version string
	URL     string
	SHA256  string
}

type DownloadFunc func(ctx context.Context, path, url string) error

type ResolveURLFunc func(ctx context.Context, url string) (string, error)

type ResolveLatestReleaseFunc func(
	ctx context.Context,
	repository,
	assetTemplate string,
) (ReleaseArtifact, error)

type GitRevision struct {
	Commit string
}

type CheckoutGitFunc func(
	ctx context.Context,
	destination string,
	extension GitExtension,
) (GitRevision, error)

type Options struct {
	Root          string
	ExternalDirs  []string
	Catalog       Catalog
	ChromeVersion string
	Download      DownloadFunc
	Resolve       ResolveURLFunc
	Verify        func(path, checksum string) error
	// VerifyCRX may replace the default CRX3 declared-ID check. The default is
	// deliberately not a complete CRX signature verifier; Chromium performs
	// that verification when it consumes the external-extension definition.
	// Chromium: https://chromium.googlesource.com/chromium/src/+/879b13b2715914eb6263fbbfa1eae7e1d49f9c1c/components/crx_file/crx_verifier.cc#155
	// Parser: https://github.com/mediabuyerbot/go-crx3/blob/v1.7.0/id.go#L22-L50
	VerifyCRX            func(path, extensionID string) error
	ResolveLatestRelease ResolveLatestReleaseFunc
	CheckoutGit          CheckoutGitFunc
	ExcludedIDs          map[string]bool
}

// Result contains command-line unpacked extension paths and any mapping from a
// catalog ID to the ID Chromium will derive from the installed key or path.
type Result struct {
	LoadExtensionPaths []string
	ExtensionIDAliases map[string]string
}
