package extensions

import "context"

type UpdatePolicy string

const (
	UpdatePolicyLatest UpdatePolicy = "latest"
	UpdatePolicyPinned UpdatePolicy = "pinned"
)

type Catalog struct {
	ChromeStoreUpdateURL string                   `toml:"chrome_store_update_url"`
	Network              NetworkConfig            `toml:"network"`
	ChromeStore          []ChromeStoreExtension   `toml:"chrome_store"`
	UpdateURL            []UpdateURLExtension     `toml:"update_url"`
	CRX                  []DownloadedExtension    `toml:"crx"`
	ZIP                  []GitHubReleaseExtension `toml:"zip"`
}

type NetworkConfig struct {
	ChromeVersion            string            `toml:"chrome_version"`
	UserAgent                string            `toml:"user_agent"`
	Headers                  map[string]string `toml:"headers"`
	TimeoutSeconds           int               `toml:"timeout_seconds"`
	RetryMax                 *int              `toml:"retry_max"`
	RetryWaitMinMilliseconds int               `toml:"retry_wait_min_milliseconds"`
	RetryWaitMaxMilliseconds int               `toml:"retry_wait_max_milliseconds"`
}

type ChromeStoreExtension struct {
	ID   string `toml:"id"`
	Name string `toml:"name"`
}

type UpdateURLExtension struct {
	ID        string `toml:"id"`
	Name      string `toml:"name"`
	UpdateURL string `toml:"update_url"`
}

type DownloadedExtension struct {
	ID      string `toml:"id"`
	Name    string `toml:"name"`
	Version string `toml:"version"`
	URL     string `toml:"url"`
	SHA256  string `toml:"sha256"`
}

type GitHubReleaseExtension struct {
	ID            string       `toml:"id"`
	Name          string       `toml:"name"`
	UpdatePolicy  UpdatePolicy `toml:"update_policy"`
	Version       string       `toml:"version"`
	URL           string       `toml:"url"`
	SHA256        string       `toml:"sha256"`
	Repository    string       `toml:"repository"`
	AssetTemplate string       `toml:"asset_template"`
	ArchiveRoot   string       `toml:"archive_root"`
	LoadUnpacked  bool         `toml:"load_unpacked"`
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

type Options struct {
	Root                 string
	ExternalDirs         []string
	Catalog              Catalog
	ChromeVersion        string
	Download             DownloadFunc
	Resolve              ResolveURLFunc
	Verify               func(path, checksum string) error
	VerifyCRX            func(path, extensionID string) error
	ResolveLatestRelease ResolveLatestReleaseFunc
	ExcludedIDs          map[string]bool
}

type Result struct {
	LoadExtensionPaths []string
	ExtensionIDAliases map[string]string
}
