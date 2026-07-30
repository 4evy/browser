<div align="center">

# `browser`

### Declarative configuration for Chromium-family browsers

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![Nix Flakes](https://img.shields.io/badge/Nix-Flakes-5277C3?style=flat-square&logo=nixos&logoColor=white)](https://nixos.wiki/wiki/Flakes)
[![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey?style=flat-square)](flake.nix)
[![MIT License](https://img.shields.io/badge/license-MIT-green?style=flat-square)](LICENSE)

</div>

`browser` applies one config to an existing Chromium-family browser. It installs
neither browsers nor hidden defaults.

| It manages        | What that means                                                         |
| ----------------- | ----------------------------------------------------------------------- |
| Launcher          | Reproducible flags, a live flags file, aliases, and unpacked extensions |
| Extensions        | Chrome Web Store, external update URLs, pinned CRXs, and release ZIPs   |
| Browser data      | `Preferences`, `Local State`, `Variations`, cookies, and shortcuts      |
| Extension data    | Ordered mutations of persistent `storage.local` and `storage.sync`      |
| Linux integration | Desktop entries, icons, Wayland/X11 IDs, and portal workarounds         |

## Quick start

Requires Go 1.26 or Nix with flakes enabled.

```sh
go build ./cmd/browser
cp browser.example.toml browser.toml

./browser configure \
  --config browser.toml \
  --mode linux \
  --root "$HOME/.cache/my-browser" \
  --app-dir /path/to/browser/app \
  --bin-dir "$HOME/.local/bin"
```

Use `--mode macos` for app bundles. An `app_dir` in the config can replace
`--app-dir`. `apply-profile-settings` skips installation,
`apply-extension-settings` changes extension storage only, and `version` prints
the version. Run `browser <command> --help` for flags.

## Configuration

[`browser.example.toml`](browser.example.toml) is the annotated TOML example:

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

Paths expand `${home}`, `${config_home}`, and `${data_home}`. Flags are layered
from config, Linux wrapper, `configure --flags`, live flags file, then launcher
arguments.

### Extension sources

| Config key                | Update behavior                                                  |
| ------------------------- | ---------------------------------------------------------------- |
| `extensions.chrome_store` | Download the newest compatible Web Store CRX                     |
| `extensions.update_url`   | Let Chromium install and update from an external manifest        |
| `extensions.crx`          | Download a fixed version and verify its SHA-256 and extension ID |
| `extensions.zip`          | Follow a GitHub release asset or verify a fully pinned ZIP       |

Store, CRX, and update-URL entries need a browser-recognized
`external_extension_dirs` path. ZIPs with `load_unpacked = true` use
`--load-extension`. `extensions.network` controls the download Chrome version,
headers, user agent, timeouts, and retries.

### Profile settings

`browser.preferences` patches `Preferences`, `Local State`, and `Variations`,
including cookies and shortcuts. Typed `browser.helium` and `browser.brave`
options cover Helium services/toolbars and Brave tabs/sidebar/Shields.

`extension_settings.files` applies ordered JSON documents to `storage.local`
and `storage.sync`: top-level writes, `set`, `merge`, `append`, `remove`, and
`clear`. CLI `--settings` files apply last; `--input` supplies runtime values.
See the [JSON schema](schema/extension-settings.schema.json).

Close the browser before applying profile or extension storage.

## Nix

Add the input, then import the appropriate module:

```nix
inputs.browser.url = "github:4evy/browser";
```

| Module system | Import                                 | Generated files                   |
| ------------- | -------------------------------------- | --------------------------------- |
| Home Manager  | `inputs.browser.homeModules.default`   | `$XDG_CONFIG_HOME/browser/*.toml` |
| NixOS         | `inputs.browser.nixosModules.default`  | `/etc/browser/*.toml`             |
| nix-darwin    | `inputs.browser.darwinModules.default` | `/etc/browser/*.toml`             |

### Complete Home Manager example

Bitwarden is real; other extension values are placeholders.

```nix
{
  inputs,
  ...
}:
{
  imports = [ inputs.browser.homeModules.default ];

  programs.browser = {
    enable = true;

    settings = {
      browser = {
        name = "Chromium";
        executable_name = "chromium";
        flags = [
          "--no-first-run"
          "--no-default-browser-check"
        ];

        paths.linux = {
          profile_dir = "\${config_home}/chromium/Default";
          external_extension_dirs = [
            "\${config_home}/chromium/External Extensions"
          ];
        };
      };

      extensions = {
        # Newest compatible Web Store release.
        chrome_store = [
          {
            id = "nngceckbapebfimnlniiiahkandclblb";
            name = "Bitwarden";
          }
        ];

        # Chromium follows the extension's update manifest.
        update_url = [
          {
            id = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
            name = "Self-hosted extension";
            update_url = "https://extensions.example.com/updates.xml";
          }
        ];

        # Fixed CRX; checksum and embedded ID are verified.
        crx = [
          {
            id = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";
            name = "Pinned CRX";
            version = "1.2.3";
            url = "https://extensions.example.com/extension-1.2.3.crx";
            sha256 = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
          }
        ];

        zip = [
          {
            id = "cccccccccccccccccccccccccccccccc";
            name = "Latest GitHub release";
            update_policy = "latest";
            repository = "owner/extension";
            asset_template = "extension-{tag}.zip";
            archive_root = "extension";
            load_unpacked = true;
          }
          {
            id = "dddddddddddddddddddddddddddddddd";
            name = "Pinned ZIP";
            update_policy = "pinned";
            version = "1.2.3";
            url = "https://extensions.example.com/extension-1.2.3.zip";
            sha256 = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
            archive_root = "extension";
            load_unpacked = true;
          }
        ];
      };

      # Nix paths become immutable store paths in the generated TOML.
      extension_settings.files = [ ./extension-settings.json ];
    };
  };
}
```

Latest GitHub assets must match `asset_template` and publish a SHA-256 digest.
Set `GITHUB_TOKEN` for private repositories or higher API limits. Unpacked
extensions automatically map storage settings to Chromium's derived ID.

Modules generate config and install the CLI; profile mutation stays explicit:

```sh
browser configure \
  --config "$XDG_CONFIG_HOME/browser/browser.toml" \
  --mode linux \
  --root "$HOME/.cache/browser" \
  --app-dir /path/to/browser/app \
  --bin-dir "$HOME/.local/bin"
```

On NixOS or nix-darwin, use `/etc/browser/browser.toml` instead.

### Multiple browsers and config-only use

`settings` generates `browser.toml`; `configurations.<name>` generates
`<name>.toml`:

```nix
programs.browser = {
  enable = true;

  configurations = {
    work.browser = {
      name = "Chromium Work";
      executable_name = "chromium-work";
    };

    brave.browser = {
      name = "Brave";
      executable_name = "brave";
    };
  };
};
```

Store paths are exposed as `configFile` and `configFiles.<name>` under
`programs.browser`. Set `package = null` to generate config without the CLI.

The flake also exports `packages.default`, `apps.default`, `overlays.default`,
and `lib.generateConfig` for use without a module.

## Development

```sh
go test ./...
nix develop
nix fmt
nix flake check
```

Released under the [MIT License](LICENSE).
