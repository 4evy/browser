# browser

Make Chromium actually declarative

[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![Nix Flakes](https://img.shields.io/badge/Nix-Flakes-5277C3?style=flat-square&logo=nixos&logoColor=white)](https://nixos.wiki/wiki/Flakes)
[![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey?style=flat-square)](flake.nix)
[![MIT License](https://img.shields.io/badge/license-MIT-green?style=flat-square)](LICENSE)

`browser` turns an existing Chromium-family browser into a reproducible setup on
macOS or Linux. One config controls its launcher, flags, profile, extensions,
and even extension settings. It changes what you declare and leaves the rest
alone

## Jump to

- [Why this exists](#why-this-exists)
- [How it compares](#how-it-compares)
- [Quick start](#quick-start)
- [Command line](#command-line)
- [Configuration](#configuration)
  - [Strip bundled services from Brave](#strip-bundled-services-from-brave)
  - [Git-backed extensions](#git-backed-extensions)
- [Nix modules](#nix-modules)
- [Development](#development)

## Why this exists

Configuring Chromium is a mess. Flags cover some settings. Enterprise policy
covers others. The rest is buried in profile JSON files and extension LevelDB
databases

`browser` pulls those pieces into one config. Use it to

- set flags and patch `Preferences`, `Local State`, and `Variations`
- install extensions from the Web Store, CRX files, release ZIPs, or Git
- configure `storage.local` and `storage.sync` without writing a LevelDB script
- keep the rest of your browser profile intact

This is why `browser` exists: installing an extension is easy. Reproducing its
settings without clobbering the rest of your profile isn't

If you only need a browser package, a few flags, and Web Store extensions, use
Home Manager. Use `browser` when you want to reproduce the setup you normally
finish by hand after installing the browser

## How it compares

The alternatives stop at one layer of the browser setup

- [Home Manager](https://github.com/nix-community/home-manager/blob/cfba7ad5886b342b8dd63ba74354b3853ea4cfc9/modules/programs/chromium.nix)
  installs a browser and declares flags, Web Store extensions, dictionaries, and
  native-messaging hosts. It stops before profile preferences and extension
  storage, which are the settings you otherwise finish by hand
- [The NixOS Chromium module](https://github.com/NixOS/nixpkgs/blob/ab9b9ebd4c1e5e4e375f602ad6b24b794cfa5661/nixos/modules/programs/chromium.nix)
  writes system-wide policy and first-run preferences. It doesn't install the
  browser, manage its live profile, or configure extension storage
- [`nix-flake-helium-browser`](https://github.com/oxcl/nix-flake-helium-browser/tree/4fe9ac832466143224203f896a3a38aa8c73b611)
  packages Helium and sets flags and policy. It targets one browser and doesn't
  manage profile or extension storage
- [`chromexup-flake`](https://github.com/crabdancing/chromexup-flake/tree/4d8d21494073bf17b77abf65cb2d03cce8c85738)
  updates Web Store extensions for ungoogled-Chromium. It doesn't manage the
  browser, support arbitrary extension sources, pin releases, or configure the
  extensions it installs

None of them covers launchers, profile data, several extension sources, and
extension settings in one model. `browser` does

`browser` doesn't install browsers. Its Nix modules don't edit a live profile
during activation, so you run `browser apply` to apply one. Pin extensions if
you need reproducible versions. Managed policy output is currently for Brave

## Quick start

You need Go 1.27 or Nix with flakes enabled. On macOS, binaries built with Go
1.27 need macOS 13 Ventura or newer

```sh
go build ./cmd/browser
cp examples/browser.toml browser.toml

./browser apply --app-dir /path/to/browser/app
```

The CLI detects macOS or Linux, keeps persistent extension and browser files
below your platform's user data directory, writes launchers to your user
executable directory (usually `~/.local/bin`), and reads the profile from the
config. Put `app_dir` in the config too and future runs become simply
`browser apply`

## Command line

Commands follow a noun-and-verb layout, while the complete declarative workflow
gets the short top-level `apply` command

```text
browser apply [CONFIG] [-- BROWSER_ARG...]
browser config validate [CONFIG ...]
browser profile apply [CONFIG]
browser storage apply [CONFIG]
browser policy render [CONFIG ...]
browser completion bash|zsh|fish|powershell
```

`browser apply` installs configured extensions, creates the launcher, and
applies profile settings. The focused commands are useful for validation, system
policy generation, and repairing one layer without reinstalling the others. Run
any command with `--help` for examples and all overrides

When `CONFIG` is omitted, `browser` uses `BROWSER_CONFIG`, then the first
`browser.toml` it finds in the current directory, the platform's user config
directories, its system config directories, or `/etc/browser`. It honors
`XDG_CONFIG_HOME` and `XDG_CONFIG_DIRS`; on macOS without an explicit
`XDG_CONFIG_HOME`, it checks both Application Support and `~/.config` so files
created by XDG-oriented tools remain discoverable.

The current platform is automatic; override it with `--platform` or
`BROWSER_PLATFORM` when cross-configuring. Persistent install files default to
`browser/<executable_name>` below the platform's user data directory and the
launcher defaults to `XDG_BIN_HOME`, or `~/.local/bin` when it is unset.
Override the two locations with `BROWSER_INSTALL_DIR`, `BROWSER_BIN_DIR`,
`--install-dir`, or `--bin-dir`. Use `--profile-dir` to apply the complete
configuration to a different profile, or `--no-profile` to skip profile changes

Put arguments stored in the generated browser launcher after the standard `--`
option boundary. Each shell argument remains exactly one browser argument, so
there is no second shell parser or nested quoting syntax

```sh
browser apply -- --disable-sync --enable-features=VerticalTabs
```

Use `-q` for silent success in scripts. Errors and diagnostics go to standard
error. Commands that produce data use standard output: for example,
`browser policy render` emits JSON there by default and accepts `-o PATH` to
write a file. Runtime JSON input accepts `--input -` for standard input

Enable tab completion for the current shell with one of

```sh
source <(browser completion bash)
source <(browser completion zsh)
browser completion fish | source
# PowerShell
browser completion powershell | Out-String | Invoke-Expression
```

The Nix package installs Bash, Zsh, and Fish completion files automatically

## Configuration

Copy and edit the [annotated example](examples/browser.toml)

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

Using Nix? Add the flake input and write the same config through its module

```nix
inputs.browser.url = "github:4evy/browser";
```

```nix
{
  imports = [ inputs.browser.homeModules.default ];

  programs.browser = {
    enable = true;
    settings.browser = {
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
}
```

Paths support `${home}`, `${config_home}`, and `${data_home}`. Each
`external_extension_dirs` entry must point to a directory the browser recognizes
as an external-extension source. `browser` tracks the definitions it writes, so
later runs remove its stale entries without touching yours

From one config, you can manage

- Chrome Web Store, external update URL, pinned CRX, release ZIP, and Git-backed
  extensions
- `Preferences`, `Local State`, `Variations`, cookies, and shortcuts
- Helium and Brave-specific preferences
- `storage.local` and `storage.sync` through ordered JSON mutations
- Linux desktop entries, icons, and portal settings

The [extension settings schema](schema/extension-settings.schema.json) defines
the storage mutations. Close the browser before changing its profile or
extension storage

The [command-line overview](#command-line) covers focused profile, storage, and
policy workflows

### Strip bundled services from Brave

Want regular Brave to behave more like Brave Origin? Start here

```toml
[browser.brave]
origin = true
```

This applies Brave's current published Origin policy defaults, copies the
sidebar and promotion defaults that configuration can reach, and turns off the
services listed below. It doesn't fake an Origin purchase, SKU, branding, or
updater identity

To keep regular Brave and just cut the bundled services and promotions, use

```toml
[browser.brave]
disable_annoyances = true
```

That turns off Wallet/provider injection, Rewards/BAT, ENS/SNS/Unstoppable
Domains, legacy IPFS/WebTorrent settings, ads, sponsored new-tab content, Leo
and local AI, VPN, News, Talk, Playlist, Web Discovery, P3A and stats pings,
email aliases, search promotions, new-tab widgets, Speedreader, Wayback Machine,
PSST, and crash/default-browser prompts. If you only want to remove crypto and
Web3, set `disable_web3 = true` instead

Set any feature explicitly to override the preset

```toml
[browser.brave]
disable_annoyances = true

[browser.brave.features]
news = true
speedreader = true
```

The same options in Nix

```nix
programs.browser.settings.browser.brave = {
  disable_annoyances = true;
  features = {
    news = true;
    speedreader = true;
  };
};
```

Most switches are ordinary `Preferences` or `Local State` values. Brave locks
its strongest Wallet, Rewards, AI, VPN, News, and telemetry controls behind
enterprise policy, so the preset generates that policy too. On Linux, install it
with

```sh
sudo browser policy render browser.toml \
  --output /etc/brave/policies/managed/browser.json
```

Brave reads `/etc/brave/policies` on Linux. On macOS, mandatory policy needs a
configuration profile or MDM. `defaults write` only sets recommended policy and
can't enforce the hard kill switches. The profile settings still hide and opt
out of the bundled services without managed policy

Need a setting that isn't typed yet? Add the raw Brave value or Chromium policy
directly. Raw values run last, so they can also override a preset

```toml
[[browser.brave.profile_values]]
path = "brave.some_new_profile_preference"
value = false

[[browser.brave.local_state_values]]
path = "brave.some_new_local_state_preference"
value = 3

[browser.brave.managed_policies]
BrowserSignin = 0
```

Between `browser.preferences.values`, `local_state_values`, `flags`, and the
Brave raw values, you can set any preference, Local State value, policy, or flag
that Brave exposes. The typed names were checked against Brave Core
[`2f817e2`](https://github.com/brave/brave-core/tree/2f817e264632a29c264f657f673d500a5c51696b)

### Git-backed extensions

Load unpacked extensions from GitHub, GitLab, Codeberg, SourceHut, or any Git
remote. For a hosted provider, a short repository path is enough

```toml
[[extensions.git]]
id = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
name = "Pinned GitLab extension"
provider = "gitlab"
repository = "group/team/extension"
update_policy = "pinned"
ref = "v1.2.3"
commit = "0123456789abcdef0123456789abcdef01234567"
subdirectory = "dist/extension"
load_unpacked = true
```

Use `github`, `gitlab`, `codeberg`, or `sourcehut` as the provider. SourceHut
repositories use `~owner/project`. For any other remote, use `provider = "git"`
and `url = "ssh://git@example.test/team/extension.git"`. If `url` is present,
you can omit the provider

Pinned entries need a full 40- or 64-character commit ID. Add its tag or branch
as `ref` to allow a shallow fetch from servers that reject direct commit
fetches. With `update_policy = "latest"`, omit `commit`; `browser` follows
`ref`, or the remote's default branch if you omit both

Outside Nix, Git must be on `PATH`. The Nix package already includes it.
`browser` exports the selected tree without hooks, submodules, or `.git`. It
doesn't build the extension, so the selected root must already contain
`manifest.json`. Private remotes use Git's non-interactive credential helper or
your SSH agent. Extension HTTP headers and `GITHUB_TOKEN` aren't passed to Git

## Nix modules

Home Manager writes `browser.toml` below `$XDG_CONFIG_HOME/browser`. For NixOS
or nix-darwin, import `nixosModules.default` or `darwinModules.default`; both
write below `/etc/browser`. Every module installs the CLI. None edits a live
profile during activation. Run `browser apply` to apply the file

Need more than one setup? Replace `settings` with `configurations.<name>`. Known
keys are typed and documented, but you can still pass through unknown
TOML-compatible keys. Set `package = null` if you only want the config file

`programs.browser.managedPolicyFile` points to the generated policy file. NixOS
also installs it at `/etc/brave/policies/managed/browser.json`, which enforces
Brave's hard-disable policies. Home Manager keeps a copy below
`$XDG_CONFIG_HOME/browser`; nix-darwin keeps one below `/etc/browser`. On macOS,
you still need to deploy it as a managed configuration profile to enforce it

The flake also exports `packages.default`, `apps.default`, `overlays.default`,
and `lib.generateConfig`. `homeManagerModules` remains as an alias for the old
spelling

## Development

```sh
nix develop
npm ci
npm run format:check
go fix -diff ./...
go vet ./...
go test -race -count=1 ./...
golangci-lint run ./...
nix fmt
deadnix --fail .
statix check .
nix flake check -L
```

Run `npm run format` to format Markdown, JSON, YAML, and JavaScript with the
project-pinned Prettier version. Go and Nix continue to use their native
formatters (`gofmt`/`gopls` and `nix fmt`). VS Code recommends those formatters,
the TOML formatter, and EditorConfig support, and formats supported files on
save. See [Go modernization and dependency audit](docs/go-modernization.md) for
the current design decisions and follow-up work

Released under the [MIT License](LICENSE)
