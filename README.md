<h1 align="center">browser</h1>

<p align="center">
  <strong>Make Chromium-family browsers declarative.</strong><br>
  Flags, preferences, extensions, and extension settings in one config.
</p>

<p align="center">
  <img
    src="https://img.shields.io/badge/macOS-000000?style=flat-square&amp;logo=apple&amp;logoColor=white"
    alt="macOS"
  >
  <img
    src="https://img.shields.io/badge/Linux-FCC624?style=flat-square&amp;logo=linux&amp;logoColor=black"
    alt="Linux"
  >
  <img
    src="https://img.shields.io/badge/Go-1.27.1%2B-00ADD8?style=flat-square&amp;logo=go&amp;logoColor=white"
    alt="Go 1.27.1 or newer"
  >
  <img
    src="https://img.shields.io/badge/Nix-flakes-5277C3?style=flat-square&amp;logo=nixos&amp;logoColor=white"
    alt="Nix flakes"
  >
</p>

Keep your browser setup in TOML or Nix: vertical tabs, cookie rules, extensions,
and the settings inside those extensions. Works with Chromium-family browsers on
macOS and Linux, with dedicated settings for Helium and Brave.

For example, the settings portion of a Helium config can look like this:

```toml
[browser.helium.appearance]
layout = "vertical"

[browser.helium.behavior]
cycle_tabs_by_most_recent_use = true

[browser.preferences.cookies]
third_party = "block"

[extensions]
chrome_store_update_url = "https://clients2.google.com/service/update2/crx"

[[extensions.chrome_store]]
id = "eimadpbcbfnmbkopoojfekhnkhdbieeh"
name = "Dark Reader"

[extension_settings]
files = ["extension-settings.json"]
```

`browser apply` installs the extensions, writes your chosen settings, and
creates a launcher with your flags. Omitted preferences stay as they are.

## Install

With Nix:

```sh
nix profile add github:4evy/browser
```

Or build with Go 1.27.1 or newer:

```sh
git clone https://github.com/4evy/browser.git
cd browser
go install ./cmd/browser
```

Add Go's install directory (`$(go env GOPATH)/bin`, or your `GOBIN`) to `PATH`.

## Get started

Install your browser and open it once to create a profile. Then:

1. Copy a starter to `browser.toml`: [Helium](examples/helium.toml),
   [Brave](examples/brave.toml), or [Chromium](examples/browser.toml).
2. Set its application and profile paths. Find **Profile Path** at
   `chrome://version` (`brave://version` in Brave); use the folder ending in
   `Default` or `Profile 1`. Each starter has macOS and Linux sections; only
   your current platform is used.
3. Quit the browser completely, then validate and apply:

   ```sh
   browser config validate browser.toml
   browser apply browser.toml
   ```

Open the launcher at the printed path, normally `~/.local/bin/helium-configured`
for the Helium starter. Use that launcher to include your flags and unpacked
extensions. After changing your config, quit, apply, and reopen through the
launcher.

The starters are complete configs. The snippets here show settings to add or
change in them; edit an existing table instead of declaring it twice.

## Configure your browser

Helium's [starter](examples/helium.toml) shows layouts, tab behavior, toolbar
buttons, privacy signals, services, and custom shortcuts.

For Brave, you can disable bundled services and enable vertical tabs:

```toml
[browser.brave]
disable_annoyances = true

[browser.brave.tabs]
vertical = true
```

The [Brave starter](examples/brave.toml) also shows feature overrides, Shields,
and managed policies. Fully disabling Wallet and Rewards requires installing the
JSON produced by `browser policy render` in Brave's managed policy store.

The [shared recipes](examples/browser.toml) cover cookie exceptions, arbitrary
Preferences and Local State values, and extension sources:

- **Chrome Web Store** and **update URLs** for store or self-hosted extensions
- **CRX** for a version pinned by SHA-256
- **ZIP** for a pinned archive or the latest GitHub release
- **Git** for a pinned commit or a followed branch, including private SSH
  remotes

ZIP and Git sources must contain a ready-to-load `manifest.json`; no build step
runs. Git sources require Git on `PATH`. Store and CRX installs use the
starter's `external_extension_dirs`; these must match your browser's discovery
locations. The browser may ask you to enable an externally installed extension.

## Configure extensions too

Save this as `extension-settings.json` beside `browser.toml` and add the
`[extension_settings]` table shown above. This is a real Dark Reader example: it
enables the extension, uses local settings, and sets dark mode with 90%
brightness and 110% contrast.

```json
{
  "schema_version": 1,
  "local": [
    {
      "id": "eimadpbcbfnmbkopoojfekhnkhdbieeh",
      "values": {
        "enabled": true,
        "syncSettings": false
      }
    }
  ],
  "operations": [
    {
      "id": "eimadpbcbfnmbkopoojfekhnkhdbieeh",
      "area": "local",
      "key": "theme",
      "operation": "merge",
      "value": {
        "mode": 1,
        "brightness": 90,
        "contrast": 110
      }
    }
  ]
}
```

`local` sets the listed storage keys. `merge` changes those theme fields while
keeping the rest of the theme. `"syncSettings": false` makes Dark Reader read
these local values, following its
[storage logic](https://github.com/darkreader/darkreader/blob/main/src/background/user-storage.ts)
and
[theme format](https://github.com/darkreader/darkreader/blob/main/src/defaults.ts).
The same JSON is available as a
[copyable example](examples/extension-settings.json).

Other extensions need their own IDs and storage keys. You can also append unique
array values, remove keys, and edit nested or compressed JSON; see the
[storage schema](schema/extension-settings.schema.json) for the full format.
`sync` writes update the local cache of `chrome.storage.sync`; existing cloud
state can overwrite them when the browser starts.

## Nix modules

Add the flake input:

```nix
inputs.browser.url = "github:4evy/browser";
```

For Home Manager on Linux:

```nix
{ inputs, pkgs, ... }:
{
  imports = [ inputs.browser.homeModules.default ];
  home.packages = [ pkgs.chromium ];

  programs.browser = {
    enable = true;
    settings.browser = {
      name = "Chromium";
      executable_name = "chromium-configured";
      flags = [ "--no-first-run" "--no-default-browser-check" ];
      linux = {
        app_dir = "${pkgs.chromium}/bin";
        launcher_name = "chromium";
      };
      paths.linux.profile_dir = "\${config_home}/chromium/Default";
      preferences.cookies.third_party = "block";
    };
  };
}
```

Pass `inputs` through Home Manager's `extraSpecialArgs`. Settings use the same
keys as TOML, including `browser.helium`, `browser.brave`, `extensions`, and
`extension_settings`. They support normal Nix module merging and overrides.

| Setup        | Import                                 |
| ------------ | -------------------------------------- |
| Home Manager | `inputs.browser.homeModules.default`   |
| NixOS        | `inputs.browser.nixosModules.default`  |
| nix-darwin   | `inputs.browser.darwinModules.default` |

The modules install the CLI and generate `browser.toml` in your user config
directory (Home Manager) or `/etc/browser` (system modules). For macOS, replace
`linux` and `paths.linux` with the `macos` paths from your browser's starter.
After activation, quit the browser and run `browser apply`, then open the
configured launcher. Use `programs.browser.configurations.<name>` for additional
configs and apply their generated `<name>.toml` explicitly.

<details>
<summary><strong>More commands and path options</strong></summary>

Without a config argument, commands check `BROWSER_CONFIG`, `./browser.toml`,
then user and system config directories. `${home}`, `${config_home}`, and
`${data_home}` in TOML expand to your user directories.

`profile_dir` selects the profile to edit. To launch a non-default profile too,
add its flag to `browser.flags`, for example `"--profile-directory=Profile 1"`.

```sh
browser profile apply browser.toml
browser storage apply browser.toml
browser policy render browser.toml --output browser-policy.json
browser apply browser.toml --app-dir /path/to/browser \
  --profile-dir /path/to/profile
browser completion zsh
```

`profile apply` writes browser and extension settings; `storage apply` writes
only extension storage. `policy render` produces JSON to install in a managed
policy store. Use `browser <command> --help` for options.

</details>

[MIT license](LICENSE)
