<div align="center">

# `browser`

### Declarative configuration for Chromium-family browsers

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![Nix Flakes](https://img.shields.io/badge/Nix-Flakes-5277C3?style=flat-square&logo=nixos&logoColor=white)](https://nixos.wiki/wiki/Flakes)
[![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey?style=flat-square)](flake.nix)
[![MIT License](https://img.shields.io/badge/license-MIT-green?style=flat-square)](LICENSE)

</div>

---

`browser` turns one TOML file into a configured Chromium-family browser.
Given an existing browser app or bundle, it can:

- create a launcher with reproducible flags;
- install Chrome Web Store, CRX, update-URL, and release ZIP extensions;
- patch Chromium `Preferences`, `Local State`, and `Variations`;
- populate extension `storage.local` and `storage.sync`; and
- install Linux desktop entries and icons.

There are no bundled browser defaults, extensions, or hidden preference
changes. The browser binary is also yours to install and update.

## Quick start

Requires Go 1.26 or Nix with flakes enabled.

```sh
go build ./cmd/browser
cp browser.example.toml browser.toml
```

Edit `browser.toml` using
[`browser.example.toml`](browser.example.toml) as a reference, then run:

```sh
./browser configure \
  --config browser.toml \
  --mode macos \
  --root "$HOME/Library/Caches/my-browser" \
  --bin-dir "$HOME/.local/bin"
```

Use `--mode linux` on Linux. If the application directory is not set in TOML,
pass it with `--app-dir`.

With Nix:

```sh
nix run . -- configure \
  --config browser.toml \
  --mode linux \
  --root "$HOME/.cache/my-browser" \
  --bin-dir "$HOME/.local/bin"
```

`configure` installs the declared extensions, applies settings to the
configured profile, and writes the launcher. Run it again whenever the
configuration changes.

## Commands

| Command                    | Purpose                                                                |
| -------------------------- | ---------------------------------------------------------------------- |
| `configure`                | Install extensions, apply profile settings, and create a launcher      |
| `apply-profile-settings`   | Apply browser preferences and extension storage to an existing profile |
| `apply-extension-settings` | Apply extension storage only                                           |
| `version`                  | Print the installed version                                            |

Every command has built-in help:

```sh
./browser configure --help
```

## Configuration

[`browser.example.toml`](browser.example.toml) is the complete starting point.
Only `browser.executable_name` is universally required; platform paths and
metadata depend on how your browser is packaged.

```toml
[browser]
name = "Chromium"
executable_name = "chromium"
flags = ["--no-first-run", "--no-default-browser-check"]

[browser.macos]
app_dir = "/Applications/Chromium.app"
launcher_path = "Contents/MacOS/Chromium"

[browser.paths.macos]
profile_dir = "${home}/Library/Application Support/Chromium/Default"
```

Path values may use `${home}`, `${config_home}`, and `${data_home}`. Relative
extension-settings paths resolve from the TOML file. Relative flags-file paths
resolve beneath the platform's XDG configuration home.

Launcher flags are applied in this order, with later layers taking precedence:
TOML `browser.flags`, platform wrapper flags, `configure --flags`, the optional
flags file, then arguments passed directly to the launcher.

### Extensions

Choose the source that matches the update policy you want:

| TOML entry                    | Behavior                                        |
| ----------------------------- | ----------------------------------------------- |
| `[[extensions.chrome_store]]` | Follow the newest compatible Web Store release  |
| `[[extensions.update_url]]`   | Register an external extension update URL       |
| `[[extensions.crx]]`          | Install a versioned, SHA-256-pinned CRX         |
| `[[extensions.zip]]`          | Follow a GitHub release or install a pinned ZIP |

Pinned downloads require a checksum. Set `GITHUB_TOKEN` when accessing private
repositories or when you need higher GitHub API limits.

### Browser and extension settings

Browser preferences are declared under `[browser.preferences]`. Dedicated
cookie-policy fields cover defaults, third-party cookies, and site exceptions;
generic path/value entries are available for Chromium-specific preferences.

Helium and Brave features have typed, opt-in sections so product preferences do
not need to be expressed as raw dotted paths:

```toml
[browser.helium.services]
enabled = true
user_consented = true
extension_proxy = true
ublock_assets = true

[browser.helium.toolbar]
show_extensions_button = false

[browser.helium]
crash_reporting = "ask" # disabled | ask | automatic

[browser.brave.tabs]
vertical = true
floating = true
on_right = false
hover_mode = "card" # tooltip | card | card_with_preview

[browser.brave.sidebar]
show = "mouseover" # always | mouseover | never

[browser.brave.shields]
adblock_only_mode = false
custom_filters = "example.com##.sponsor"
```

When Helium is configured with `--set-user-color=R,G,B`, the launcher flag is
also persisted to the profile's browser and extension theme preferences.

Extension storage is described by ordered JSON files:

```toml
[extension_settings]
files = ["settings/base.json", "settings/work.json"]
```

The format supports top-level values plus `set`, `merge`, `append`, `remove`,
and `clear` operations for `storage.local` and `storage.sync`. Runtime values
can be supplied with `--input`. See
[`schema/extension-settings.schema.json`](schema/extension-settings.schema.json)
for the complete format.

Later settings files override or extend earlier files. Files passed with
repeatable `--settings` arguments are applied after the files declared in
TOML.

Close the browser before changing profile or extension storage. All settings
documents are parsed and validated before mutation, and locked extension
storage is reported as an error.

## Nix modules

The flake exports Home Manager, NixOS, and nix-darwin modules:

```nix
inputs.browser.url = "github:4evy/browser";

# Home Manager
imports = [ inputs.browser.homeModules.default ];
```

Use `nixosModules.default` or `darwinModules.default` instead when NixOS or
nix-darwin owns the configuration. Then configure it with:

```nix
{
  programs.browser = {
    enable = true;

    settings.browser = {
      name = "Chromium";
      executable_name = "chromium";
      paths.linux.profile_dir = "\${config_home}/chromium/Default";
    };

    configurations.brave.browser = {
      name = "Brave";
      executable_name = "brave";
      paths.linux.profile_dir =
        "\${config_home}/BraveSoftware/Brave-Browser/Default";
    };
  };
}
```

Home Manager writes files to `$XDG_CONFIG_HOME/browser`; NixOS and nix-darwin
write them to `/etc/browser`. Modules generate configuration and install the
CLI only—profile mutation remains an explicit `browser` command.

Set `package = null` to generate configuration without installing the CLI.
The flake also exports `overlays.default` and `lib.generateConfig`.

## Development

```sh
go test ./...

# Or run the full Nix checks:
nix develop
nix fmt
nix flake check
```

Released under the [MIT License](LICENSE).
