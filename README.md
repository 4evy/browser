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
examples below describe the same Linux setup, including a Chrome Web Store
extension.

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

[[extensions.chrome_store]]
id = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
name = "Example extension"
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

      extensions.chrome_store = [
        {
          id = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
          name = "Example extension";
        }
      ];
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

The annotated example includes preferences, extension sources, extension
storage, Helium settings, and Brave settings. The
[extension settings schema](schema/extension-settings.schema.json) documents the
JSON mutation format for `storage.local` and `storage.sync`.

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
