# browser

Make Chromium-family browsers declarative.

`browser` applies one TOML configuration to an existing browser on macOS or
Linux. It can manage:

- launchers and flags
- Chromium profile preferences, cookies, and shortcuts
- extensions from the Chrome Web Store, CRX files, release archives, or Git
- extension `storage.local` and `storage.sync`
- Helium settings and Brave features and policies

It changes the values it owns and leaves the rest of the profile alone. It does
not install the browser itself.

## Quick start

Build with Go 1.27, then copy the annotated configuration:

```sh
go build ./cmd/browser
cp examples/browser.toml browser.toml
```

Set the browser application and profile paths in `browser.toml`, close the
browser, and apply the configuration:

```sh
./browser apply
```

The first run can take the application path from the command line instead:

```sh
./browser apply --app-dir /path/to/browser/app
```

`browser apply` detects the current platform, installs the configured
extensions, creates a launcher, and applies the profile settings.

## Configuration

Start with [the annotated example](examples/browser.toml). The TOML and Nix
examples below describe the same Linux setup.

### TOML

```toml
[browser]
name = "Chromium"
executable_name = "chromium"
flags = ["--no-first-run", "--no-default-browser-check"]

[browser.linux]
app_dir = "/opt/chromium"
launcher_name = "chromium"

[browser.paths.linux]
profile_dir = "${config_home}/chromium/Default"
external_extension_dirs = ["${config_home}/chromium/External Extensions"]
```

### Nix

Add the flake input:

```nix
inputs.browser.url = "github:4evy/browser";
```

```nix
{
  imports = [ inputs.browser.homeModules.default ];

  programs.browser = {
    enable = true;
    settings = {
      browser = {
        name = "Chromium";
        executable_name = "chromium";
        flags = [ "--no-first-run" "--no-default-browser-check" ];

        linux = {
          app_dir = "/opt/chromium";
          launcher_name = "chromium";
        };

        paths.linux = {
          profile_dir = "\${config_home}/chromium/Default";
          external_extension_dirs = [
            "\${config_home}/chromium/External Extensions"
          ];
        };
      };
    };
  };
}
```

Home Manager writes the configuration to `$XDG_CONFIG_HOME/browser`. The flake
also provides NixOS and nix-darwin modules as `nixosModules.default` and
`darwinModules.default`. Modules install the CLI and write the configuration,
but do not edit a live profile during activation. Run `browser apply` after
activation.

Paths in either format support `${home}`, `${config_home}`, and `${data_home}`.
Close the browser before changing its profile or extension storage.

## Extensions

Every extension source works in TOML and through the Nix modules. These
equivalent catalogs show each source type.

### TOML

```toml
[extensions]
chrome_store_update_url = "https://clients2.google.com/service/update2/crx"

[[extensions.chrome_store]]
id = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
name = "Chrome Web Store extension"

[[extensions.update_url]]
id = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
name = "Extension with an update manifest"
update_url = "https://example.test/extension/updates.xml"

[[extensions.crx]]
id = "cccccccccccccccccccccccccccccccc"
name = "Pinned CRX"
version = "1.2.3"
url = "https://example.test/extension-1.2.3.crx"
sha256 = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

[[extensions.zip]]
id = "dddddddddddddddddddddddddddddddd"
name = "Latest GitHub release"
update_policy = "latest"
repository = "owner/repository"
asset_template = "extension-{tag}.zip"
archive_root = "extension"
load_unpacked = true

[[extensions.zip]]
id = "ffffffffffffffffffffffffffffffff"
name = "Pinned ZIP"
update_policy = "pinned"
version = "1.2.3"
url = "https://example.test/extension-1.2.3.zip"
sha256 = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
archive_root = "extension"
load_unpacked = true

[[extensions.git]]
id = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
name = "Pinned Codeberg extension"
provider = "codeberg"
repository = "owner/extension"
update_policy = "pinned"
ref = "v1.2.3"
commit = "0123456789abcdef0123456789abcdef01234567"
subdirectory = "dist/extension"
load_unpacked = true

[[extensions.git]]
id = "gggggggggggggggggggggggggggggggg"
name = "Latest generic Git extension"
provider = "git"
url = "ssh://git@example.test/team/extension.git"
update_policy = "latest"
ref = "main"
load_unpacked = true
```

### Nix

```nix
programs.browser.settings.extensions = {
  chrome_store_update_url = "https://clients2.google.com/service/update2/crx";

  chrome_store = [
    {
      id = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
      name = "Chrome Web Store extension";
    }
  ];

  update_url = [
    {
      id = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";
      name = "Extension with an update manifest";
      update_url = "https://example.test/extension/updates.xml";
    }
  ];

  crx = [
    {
      id = "cccccccccccccccccccccccccccccccc";
      name = "Pinned CRX";
      version = "1.2.3";
      url = "https://example.test/extension-1.2.3.crx";
      sha256 = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
    }
  ];

  zip = [
    {
      id = "dddddddddddddddddddddddddddddddd";
      name = "Latest GitHub release";
      update_policy = "latest";
      repository = "owner/repository";
      asset_template = "extension-{tag}.zip";
      archive_root = "extension";
      load_unpacked = true;
    }
    {
      id = "ffffffffffffffffffffffffffffffff";
      name = "Pinned ZIP";
      update_policy = "pinned";
      version = "1.2.3";
      url = "https://example.test/extension-1.2.3.zip";
      sha256 = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
      archive_root = "extension";
      load_unpacked = true;
    }
  ];

  git = [
    {
      id = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee";
      name = "Pinned Codeberg extension";
      provider = "codeberg";
      repository = "owner/extension";
      update_policy = "pinned";
      ref = "v1.2.3";
      commit = "0123456789abcdef0123456789abcdef01234567";
      subdirectory = "dist/extension";
      load_unpacked = true;
    }
    {
      id = "gggggggggggggggggggggggggggggggg";
      name = "Latest generic Git extension";
      provider = "git";
      url = "ssh://git@example.test/team/extension.git";
      update_policy = "latest";
      ref = "main";
      load_unpacked = true;
    }
  ];
};
```

Chrome Web Store entries follow the latest compatible release. CRX entries are
checksum-pinned. ZIP entries can follow a GitHub release or use a pinned URL and
checksum.

Git sources support GitHub, GitLab, Codeberg, and SourceHut through `provider`
and `repository`. Use `provider = "git"` with `url` for any other Git remote.
`update_policy = "latest"` follows `ref` or the default branch; pinned entries
require a full commit ID.

### Extension settings

Extension settings are ordered JSON documents applied to `storage.local` and
`storage.sync`:

```toml
[extension_settings]
files = ["settings/privacy.json", "settings/work.json"]
```

```nix
programs.browser.settings.extension_settings.files = [
  ./settings/privacy.json
  ./settings/work.json
];
```

See the [extension settings schema](schema/extension-settings.schema.json) for
the JSON mutation format. The annotated example also covers download headers,
timeouts, retries, and pinned ZIP files.

### Brave

To disable Brave's bundled services and promotions, use either format:

```toml
[browser.brave]
disable_annoyances = true
```

```nix
programs.browser.settings.browser.brave = {
  disable_annoyances = true;
};
```

Use `disable_web3 = true` for only crypto and Web3 features, or `origin = true`
to apply Brave Origin's published policy and UI defaults. Explicit feature
settings override these presets.

Some Brave controls require managed policy. On Linux, render it to Brave's
managed policy directory:

```sh
sudo browser policy render browser.toml \
  --output /etc/brave/policies/managed/browser.json
```

## Commands

```text
browser apply [CONFIG] [-- BROWSER_ARG...]
browser config validate [CONFIG ...]
browser profile apply [CONFIG]
browser storage apply [CONFIG]
browser policy render [CONFIG ...]
browser completion bash|zsh|fish|powershell
```

Without `CONFIG`, `browser` uses `BROWSER_CONFIG` or discovers `browser.toml` in
the standard user and system configuration directories. Use
`browser <command> --help` for command options and examples.

## Development

```sh
nix develop
npm ci
npm run format:check
go test -race -count=1 ./...
nix flake check -L
```

Released under the [MIT License](LICENSE).
