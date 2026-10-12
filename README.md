<div align="center">
  <h1>Tweezle</h1>
  <p>
    <strong>Declarative browser configuration, down to your extensions</strong>
  </p>
  <p>Keep your browser setup in your dotfiles. Apply it with TOML or Nix.</p>
  <p>
    <img src="https://img.shields.io/badge/platform-Linux%20%7C%20macOS-5856d6" alt="Linux and macOS">
    <a href="#nix"><img src="https://img.shields.io/badge/Nix-modules-5277c3" alt="Nix modules"></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-a6da95" alt="MIT license"></a>
  </p>
  <p>
    <a href="#install">Install</a> ·
    <a href="#quick-start">Quick start</a> ·
    <a href="#nix">Nix</a> ·
    <a href="examples/">Examples</a>
  </p>
</div>

Describe the setup you want, then run `tweezle apply`. Tweezle configures
installed Chromium and Firefox-family browsers on Linux and macOS, including
forks such as Brave, Helium, and Zen.

- **Bring extension settings along.** Manage Chromium `local` and `sync`
  storage, import uBlock Origin backups, and set Firefox managed extension
  values where the extension supports them.
- **Choose your extension sources.** Install Chromium extensions from the Chrome
  Web Store, update manifests, CRX, ZIP, or Git. Pin supported sources to exact
  content or follow updates.
- **Configure the browser you actually use.** Set preferences, cookies, and
  launch flags, plus fork-specific features such as Helium's vertical tabs,
  Brave Shields, and Zen workspaces and mods.
- **Fit it into your dotfiles.** Use the standalone TOML CLI or the Home
  Manager, NixOS, and nix-darwin modules. Native and Flatpak installations have
  [example configurations](examples/).

## Install

From a checkout, use Go 1.27.1 or newer, or Nix with flakes enabled:

```sh
go install ./cmd/tweezle
```

Add Go's binary directory (`$(go env GOPATH)/bin` by default) to your `PATH`.
For Nix, use `nix profile add .` to install or `nix run . -- --help` to try it.
The flake provides packages for x86_64 and aarch64 Linux, and aarch64 macOS.

## Quick start

Open your browser once, then find its profile path at `chrome://version`
(Chromium) or `about:profiles` (Firefox). Save **one** of these examples as
`tweezle.toml` and adapt its application and profile paths.

<details open>
<summary><strong>Chromium: keep cookies only for the session</strong></summary>

```toml
[browser]
name = "Chromium"
executable_name = "chromium-configured"
flags = ["--no-first-run"]

[browser.linux]
app_dir = "/path/to/chromium"
launcher_name = "chrome"

[browser.paths.linux]
profile_dir = "${config_home}/chromium/Default"

[browser.macos]
app_dir = "/Applications/Chromium.app"
launcher_path = "Contents/MacOS/Chromium"

[browser.paths.macos]
profile_dir = "${home}/Library/Application Support/Chromium/Default"

[browser.preferences.cookies]
default = "session_only"
third_party = "block"
```

On Linux, `app_dir` and `launcher_name` must point to your installed browser
executable. For another Chromium profile, also add its
`--profile-directory=Profile 1` flag to `browser.flags`.

</details>

<details>
<summary><strong>Firefox: restore your previous session</strong></summary>

```toml
[firefox]
variant = "firefox"

[firefox.paths.linux]
profile_dir = "${config_home}/mozilla/firefox/REPLACE.default-release"

[firefox.paths.macos]
profile_dir = "${home}/Library/Application Support/Firefox/Profiles/REPLACE.default-release"

[firefox.preferences]
"browser.startup.page" = 3
"browser.tabs.warnOnClose" = false
```

Firefox must be on `PATH` on Linux; the default macOS application path is
`/Applications/Firefox.app`. Preferences are written to a managed block in
`user.js`.

</details>

Close the browser before applying so it cannot overwrite profile changes:

```sh
tweezle config validate tweezle.toml
tweezle apply tweezle.toml
```

`apply` selects the current platform and prints the generated launcher path. Use
that launcher for configured flags and unpacked Chromium extensions. Without a
file argument, Tweezle checks `TWEEZLE_CONFIG`, `./tweezle.toml`, then user and
system configuration directories.

## Nix

`programs.tweezle.settings` configures the default browser setup. Choose
`settings.chromium` or `settings.firefox`; both generate `tweezle/tweezle.toml`.
Chromium's `extensions`, `extension_settings`, and `ublock` options belong
inside `settings.chromium` too.

Add `inputs.tweezle.url = "github:4evy/tweezle";` to your flake. For Home
Manager, pass `extraSpecialArgs = { inherit inputs; };` to
`homeManagerConfiguration`. Choose one of these Linux modules and adapt the
profile path using `chrome://version` or `about:profiles`.

<details open>
<summary><strong>Chromium settings</strong></summary>

```nix
{ inputs, pkgs, ... }:
{
  imports = [ inputs.tweezle.homeModules.default ];
  home.packages = [ pkgs.chromium ];

  programs.tweezle = {
    enable = true;
    settings.chromium = {
      name = "Chromium";
      executable_name = "chromium-configured";
      linux = {
        app_dir = "${pkgs.chromium}/bin";
        launcher_name = "chromium";
      };
      paths.linux.profile_dir = "\${config_home}/chromium/Default";
      preferences.cookies = {
        default = "session_only";
        third_party = "block";
      };
    };
  };
}
```

</details>

<details>
<summary><strong>Firefox settings</strong></summary>

```nix
{ inputs, pkgs, ... }:
{
  imports = [ inputs.tweezle.homeModules.default ];
  home.packages = [ pkgs.firefox ];

  programs.tweezle = {
    enable = true;
    settings.firefox = {
      variant = "firefox";
      paths.linux.profile_dir = "\${config_home}/mozilla/firefox/REPLACE.default-release";
      preferences."browser.startup.page" = 3;
    };
  };
}
```

</details>

Close the affected browsers before switching your Home Manager configuration.
The module installs Tweezle, writes `$XDG_CONFIG_HOME/tweezle/tweezle.toml`
(`~/.config/tweezle/tweezle.toml` by default), and runs `tweezle apply` after
linking the generation. This creates the configured launchers and updates the
configured profiles. Home Manager dry runs print the apply commands without
running them.

NixOS and nix-darwin expose the same `programs.tweezle` options through
`inputs.tweezle.nixosModules.default` and
`inputs.tweezle.darwinModules.default`, writing configuration under
`/etc/tweezle/`. NixOS applies the configurations through user activation:
running user sessions receive updates on a switch, and other users receive them
when they next log in. nix-darwin applies them during the switch as
`system.primaryUser`; set that option to your username. Both run Tweezle as the
target user so files in their home directories retain the correct ownership.

Set `programs.tweezle.activate = false;` to keep automatic application off and
run `tweezle apply` yourself. Setting `programs.tweezle.package = null;` also
disables activation by default and manages configuration files without
installing Tweezle.

### Named configurations (optional)

To keep another setup alongside `settings`, add a named configuration to your
Home Manager modules. In this example, `work` is the file name and `firefox` is
the browser family:

```nix
{ pkgs, ... }:
{
  home.packages = [ pkgs.firefox ];
  programs.tweezle.configurations.work.firefox = {
    variant = "firefox";
    paths.linux.profile_dir = "\${config_home}/mozilla/firefox/REPLACE.default-release";
    preferences."browser.startup.page" = 3;
  };
}
```

This adds `tweezle/work.toml`; your default `tweezle.toml` still comes from
`settings`. Activation applies both the default and every named configuration,
so close all affected browsers before switching. When automatic activation is
disabled, apply this file with
`tweezle apply "${XDG_CONFIG_HOME:-$HOME/.config}/tweezle/work.toml"`.

## Examples

Browse [examples/](examples/) for starter configurations and reusable recipes.

| What you want to configure | Examples |
| --- | --- |
| Chromium and its forks | [Shared recipes](examples/tweezle.toml), [Helium](examples/helium.toml), [Brave](examples/brave.toml), [Chrome](examples/chrome.toml), [Edge](examples/edge.toml), [Vivaldi](examples/vivaldi.toml), [ungoogled-chromium](examples/ungoogled.toml) |
| Firefox and its forks | [Firefox](examples/firefox.toml), [LibreWolf](examples/librewolf.toml), [Waterfox](examples/waterfox.toml), [Floorp](examples/floorp.toml), [Zen](examples/zen.toml) |
| Flatpak browsers | [Chromium](examples/flatpak-chromium.toml), [Firefox](examples/flatpak-firefox.toml) |
| Extension settings | [Dark Reader, Stylus, SponsorBlock, and more](examples/extensions/extensions.toml), [storage JSON](examples/storage/storage.json), [uBlock Origin and Lite](examples/ublock/ublock.toml) |
| Embedding Tweezle in Go | [Consumer example](examples/consumer/) |

Licensed under [MIT](LICENSE).
