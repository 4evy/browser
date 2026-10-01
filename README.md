<h1 align="center">browser</h1>

<p align="center">
  <strong>Make Chromium-family browsers declarative.</strong><br>
  Launchers, flags, preferences, extensions, and storage in one config.
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

<p align="center">
  <a href="#install">Install</a> ·
  <a href="#use">Use</a> ·
  <a href="#nix-modules">Nix modules</a> ·
  <a href="#examples">Examples</a>
</p>

Keep your browser setup in a file you can edit, reuse, and bring to another
machine. `browser` configures Chromium-family browsers, including Helium and
Brave, and updates only the profile values you choose.

Start with a few launch flags, then add preferences or extensions when you need
them. You can write your setup in TOML or generate it with Nix.

## Install

With Nix and flakes enabled:

```sh
nix profile add github:4evy/browser
```

Or build from source with Git and Go 1.27.1 or newer:

```sh
git clone https://github.com/4evy/browser.git
cd browser
go install ./cmd/browser
export PATH="$(go env GOPATH)/bin:$PATH"
```

The last line makes the command available in your current shell with Go's
default settings. If you set `GOBIN`, add that directory to `PATH` instead. Git
is also required at runtime if you configure Git extension sources.

## Use

### 1. Get your browser ready

Use your existing Chromium-family browser, or install
[Helium](https://helium.computer/) or [Brave](https://brave.com/download/) using
their installation instructions. On Linux, install Chromium through your
distribution's package manager; for Nix, see the [module example](#nix-modules)
below. `browser` configures that installation; it does not install the browser
itself.

Open it once to create a profile. Visit `chrome://version` and note the
[**Profile Path**](https://chromium.googlesource.com/chromium/src/+/HEAD/docs/user_data_dir.md#Current-Location)
(use `brave://version` in Brave), then quit the browser completely. The profile
path ends in something like `Default` or `Profile 1`; use that folder, rather
than its parent.

### 2. Write a small config

Start from [the Helium config](examples/helium.toml),
[the Brave config](examples/brave.toml), or the generic Chromium example below.
Copy your chosen config to `browser.toml` and change the application and profile
paths to match your installation. The Linux application path below is a
placeholder, not an installation location:

```toml
[browser]
name = "Chromium"
executable_name = "chromium-configured"
flags = ["--no-first-run", "--no-default-browser-check"]

[browser.macos]
app_dir = "/Applications/Chromium.app"
launcher_path = "Contents/MacOS/Chromium"

[browser.paths.macos]
profile_dir = "${home}/Library/Application Support/Chromium/Default"
external_extension_dirs = [
  "${home}/Library/Application Support/Chromium/External Extensions",
]

[browser.linux]
app_dir = "/path/to/directory/containing/chromium"
launcher_name = "chromium"

[browser.paths.linux]
profile_dir = "${config_home}/chromium/Default"
external_extension_dirs = ["${config_home}/chromium/External Extensions"]
```

Only the section for your current platform is used. `app_dir` is the directory
containing the installed browser, and `launcher_path` or `launcher_name` points
to its executable inside that directory. `executable_name` names the launcher
this tool creates; here, `chromium-configured` keeps it easy to distinguish from
the original browser command.

For Helium or Brave, start with the [browser-specific examples](#examples) and
adapt their paths. `${home}`, `${config_home}`, and `${data_home}` expand to
your user directories. The shared example has more settings when you're ready
for them.

### 3. Apply it and open the browser

```sh
browser config validate browser.toml
browser apply browser.toml
```

Validation checks your config without changing anything. `apply` installs any
configured extensions, updates profile settings, and prints the launcher path.
It does not open a browser window. With the default launcher directory, open
your configured browser with:

```sh
~/.local/bin/chromium-configured
```

Use the printed path if you changed the launcher directory. If `~/.local/bin` is
on your `PATH`, you can just run `chromium-configured`. Opening the original
application directly skips the launcher's flags.

The example uses the browser's default profile. `profile_dir` selects which
profile to edit; it does not make the launcher switch profiles. If you use
`Profile 1`, also add `"--profile-directory=Profile 1"` to `browser.flags` so
the launcher opens that profile.

### 4. Make it yours

For example, append this to `browser.toml` to block third-party cookies:

```toml
[browser.preferences.cookies]
third_party = "block"
```

Or add [Dark Reader](https://darkreader.org/) from the Chrome Web Store:

```toml
[extensions]
chrome_store_update_url = "https://clients2.google.com/service/update2/crx"

[[extensions.chrome_store]]
id = "eimadpbcbfnmbkopoojfekhnkhdbieeh"
name = "Dark Reader"
```

The extension ID is the last part of its Chrome Web Store URL. The
`external_extension_dirs` in the starting config let the browser discover
installed extensions; these directories must match your browser's locations.
Your browser may ask you to enable an externally installed extension.

After changing your config, quit the browser, run `browser apply browser.toml`
again, and reopen it through the launcher. Flags take effect when you launch;
profile settings are written when you apply.

<details>
<summary><strong>Config discovery and focused commands</strong></summary>

Without a config argument, commands check `BROWSER_CONFIG`, `./browser.toml`,
then the user and system configuration directories. You can also override paths
for one apply:

```sh
browser apply browser.toml --app-dir /path/to/browser/app \
  --profile-dir /path/to/browser/profile
```

```sh
browser profile apply browser.toml
browser storage apply browser.toml
browser policy render browser.toml --output browser-policy.json
browser completion zsh
```

`profile apply` updates profile and extension settings; `storage apply` updates
only extension storage. `policy render` writes JSON for a managed policy store;
you must install it in the browser's policy directory. Use
`browser <command> --help` for options.

</details>

### Examples

Copy one TOML starter as `browser.toml`, for example
`cp examples/helium.toml browser.toml`, then adapt its paths and settings:

| Example                                               | What it shows                                                                                                  |
| ----------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| [Chromium and shared recipes](examples/browser.toml)  | Launcher setup, cookies, arbitrary preferences, all five extension source types, download options, and storage |
| [Helium](examples/helium.toml)                        | Vertical tabs, recent-tab cycling, privacy signals, optional services, and custom shortcuts                    |
| [Brave](examples/brave.toml)                          | A service preset, vertical tabs, feature overrides, Shields, and managed policies                              |
| [Extension storage](examples/extension-settings.json) | Hypothetical `storage.local` and `storage.sync` values, object merges, and unique array appends                |

Each TOML file is a standalone configuration with macOS and Linux paths. The
Chromium starter enables only launcher setup; Helium and Brave also enable the
settings described at the top of their files. Copy shared recipes into your
chosen config; `apply` accepts one file. Extension source IDs, URLs, checksums,
and commits are placeholders to replace before use.

For storage settings, copy the JSON example beside your config, adapt its ID and
keys to your extension, and enable the `[extension_settings]` recipe. Relative
settings paths resolve from the TOML file. The
[JSON schema](schema/extension-settings.schema.json) describes the full mutation
format. Brave's example also requires installing the rendered managed policy to
fully disable Wallet and Rewards.

## Nix modules

Add this input to your flake:

```nix
inputs.browser.url = "github:4evy/browser";
```

If you manage your Linux setup with Home Manager, install Chromium from Nixpkgs
and point the configurator at its packaged launcher:

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
    };
  };
}
```

Pass `inputs` through Home Manager's `extraSpecialArgs` if it is not already
available to your modules. For macOS, use `macos` and `paths.macos` with the
application bundle, launcher, and profile paths from your chosen TOML example.

| Configuration | Import                                 | Config location                         |
| ------------- | -------------------------------------- | --------------------------------------- |
| Home Manager  | `inputs.browser.homeModules.default`   | `$XDG_CONFIG_HOME/browser/browser.toml` |
| NixOS         | `inputs.browser.nixosModules.default`  | `/etc/browser/browser.toml`             |
| nix-darwin    | `inputs.browser.darwinModules.default` | `/etc/browser/browser.toml`             |

All three modules use `programs.browser`. They install the CLI and generate
configuration files. After rebuilding or activating, close the browser and run:

```sh
browser apply
~/.local/bin/chromium-configured
```

The first command reads the generated config and creates your launcher; the
second opens Chromium with your flags. To add the cookie setting from above, use
`programs.browser.settings.browser.preferences.cookies.third_party = "block";`
in your module.

Keys inside `settings` match TOML, including snake_case names such as
`executable_name`; module controls such as `checkConfig` use lowerCamelCase.
Settings merge through the Nix module system and support `lib.mkDefault`,
`lib.mkForce`, and list ordering. Use `programs.browser.configurations.<name>`
for additional configs, then apply the generated `<name>.toml` explicitly.

<p align="center">
  <a href="LICENSE">
    <img
      src="https://img.shields.io/badge/License-MIT-A6E3A1?style=flat-square"
      alt="MIT License"
    >
  </a>
</p>
