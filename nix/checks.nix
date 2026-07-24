{
  browserPackage,
  darwinModule,
  formatterPackage,
  homeManager,
  homeManagerModule,
  lib,
  nixDarwin,
  nixosModule,
  pkgs,
  src,
}:

let
  sampleExtensionSettings = pkgs.writeText "browser-extension-settings.json" (
    builtins.toJSON {
      schema_version = 1;
      local = [
        {
          id = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
          values.module_generated = true;
        }
      ];
    }
  );

  sampleSettings = {
    browser = {
      name = "Example Browser";
      executable_name = "example-browser";
      flags_file = "example-browser-flags.conf";
      flags = [ "--no-first-run" ];
      user_agent = "Example Browser/1.0";
      future_compatible_setting = "accepted";

      linux = {
        desktop_id = "example-browser";
        portal_app_id = "org.chromium.Chromium";
      };

      paths.${if pkgs.stdenv.hostPlatform.isDarwin then "macos" else "linux"} = {
        profile_dir = "\${home}/.example-browser/Default";
        external_extension_dirs = [ "\${home}/.example-browser/External Extensions" ];
      };

      preferences = {
        values = [
          {
            path = "browser.example.enabled";
            value = true;
          }
        ];
        cookies = {
          default = "session_only";
          third_party = "block";
          allow = [ "[*.]example.com" ];
          block = [ "[*.]tracker.example" ];
          session_only = [ "[*.]session.example" ];
        };
      };

      helium = {
        crash_reporting = "ask";
        services = {
          enabled = true;
          user_consented = true;
          extension_proxy = true;
          ublock_assets = true;
        };
      };

      brave = {
        tabs = {
          vertical = true;
          hover_mode = "card";
        };
        sidebar.show = "mouseover";
        shields.adblock_only_mode = false;
      };
    };

    extensions = {
      chrome_store_update_url = "https://clients2.google.com/service/update2/crx";
      network = {
        chrome_version = "152.0.7971.0";
        user_agent = "Example Downloader/1.0";
        retry_max = 0;
      };
      chrome_store = [
        {
          id = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
          name = "Example extension";
        }
      ];
      zip = [
        {
          id = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";
          name = "Pinned example";
          update_policy = "pinned";
          version = "1.2.3";
          url = "https://example.test/extension-1.2.3.zip";
          sha256 = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
          archive_root = "extension";
          load_unpacked = true;
        }
      ];
    };

    extension_settings.files = [ sampleExtensionSettings ];
  };

  namedSettings = {
    browser = {
      name = "Second Browser";
      executable_name = "second-browser";
    };
  };

  moduleConfiguration = {
    programs.browser = {
      enable = true;
      package = browserPackage;
      settings = sampleSettings;
      configurations.second = namedSettings;
    };
  };

  homeBase = {
    home = {
      username = "browser-test";
      homeDirectory =
        if pkgs.stdenv.hostPlatform.isDarwin then "/Users/browser-test" else "/home/browser-test";
      stateVersion = "25.11";
    };
  };

  mkHome =
    module:
    homeManager.lib.homeManagerConfiguration {
      inherit pkgs;
      modules = [
        homeManagerModule
        module
        homeBase
      ];
    };

  verifyGeneratedFiles =
    name: defaultFile: namedFile:
    pkgs.runCommandLocal name { } ''
      grep -F 'executable_name = "example-browser"' ${defaultFile}
      grep -F 'portal_app_id = "org.chromium.Chromium"' ${defaultFile}
      grep -F 'crash_reporting = "ask"' ${defaultFile}
      grep -F 'hover_mode = "card"' ${defaultFile}
      grep -F 'adblock_only_mode = false' ${defaultFile}
      grep -F '${sampleExtensionSettings}' ${defaultFile}
      grep -F 'executable_name = "second-browser"' ${namedFile}

      for configFile in ${defaultFile} ${namedFile}; do
        profile="$TMPDIR/profile-$(basename "$configFile" .toml)"
        ${lib.getExe browserPackage} apply-profile-settings \
          --config "$configFile" \
          --profile-dir "$profile"
        if [ "$(basename "$configFile")" = "browser.toml" ]; then
          test -d "$profile/Local Extension Settings/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        fi
      done

      touch "$out"
    '';

  home = mkHome moduleConfiguration;

  homeDefaultFile = home.config.xdg.configFile."browser/browser.toml".source;
  homeNamedFile = home.config.xdg.configFile."browser/second.toml".source;

  homeWithoutSettings = mkHome {
    programs.browser = {
      enable = true;
      package = null;
    };
  };

  homeWithDefaultPackage = mkHome {
    programs.browser.enable = true;
  };

  homeConfigOnly = mkHome {
    programs.browser = {
      enable = true;
      package = null;
      settings = namedSettings;
    };
  };

  homeWithInvalidName = mkHome {
    programs.browser = {
      enable = true;
      package = null;
      configurations."../escape" = namedSettings;
    };
  };

  invalidNameEvaluation = builtins.tryEval homeWithInvalidName.config.home.username;

  homeWithIncompleteSettings = mkHome {
    programs.browser = {
      enable = true;
      package = null;
      settings.browser.name = "Missing executable name";
    };
  };

  incompleteSettingsEvaluation = builtins.tryEval homeWithIncompleteSettings.config.programs.browser.configFile;

  wrongClass = builtins.tryEval (
    (lib.evalModules {
      class = "nixos";
      specialArgs = { inherit pkgs; };
      modules = [ darwinModule ];
    }).config.programs.browser.enable
  );

  formatting =
    pkgs.runCommandLocal "browser-formatting"
      {
        nativeBuildInputs = [
          pkgs.findutils
          pkgs.go
          formatterPackage
        ];
      }
      ''
        cp -R ${src} source
        chmod -R u+w source

        unformatted="$(find source -name '*.go' -type f -print0 | xargs -0 gofmt -l)"
        if [ -n "$unformatted" ]; then
          echo "Go files need formatting:" >&2
          echo "$unformatted" >&2
          exit 1
        fi

        (
          cd source
          treefmt --ci --walk filesystem --tree-root "$PWD"
        )
        touch "$out"
      '';

  extensionSettingsSchema =
    pkgs.runCommandLocal "browser-extension-settings-schema"
      {
        nativeBuildInputs = [ pkgs.check-jsonschema ];
      }
      ''
        check-jsonschema \
          --schemafile ${src}/schema/extension-settings.schema.json \
          ${src}/testdata/extension-settings/comprehensive.json
        touch "$out"
      '';

in
{
  inherit extensionSettingsSchema formatting;

  module-classes =
    assert !wrongClass.success;
    pkgs.runCommandLocal "browser-module-classes" { } "touch $out";

  module-home-manager =
    assert lib.elem browserPackage home.config.home.packages;
    assert home.config.programs.browser.configFile == homeDefaultFile;
    assert home.config.programs.browser.configFiles.second == homeNamedFile;
    assert homeWithoutSettings.config.programs.browser.configFile == null;
    assert homeWithoutSettings.config.programs.browser.configFiles == { };
    assert !builtins.hasAttr "browser/browser.toml" homeWithoutSettings.config.xdg.configFile;
    assert lib.isDerivation homeWithDefaultPackage.config.programs.browser.package;
    assert lib.elem homeWithDefaultPackage.config.programs.browser.package
      homeWithDefaultPackage.config.home.packages;
    assert !lib.elem browserPackage homeConfigOnly.config.home.packages;
    assert builtins.hasAttr "browser/browser.toml" homeConfigOnly.config.xdg.configFile;
    assert !invalidNameEvaluation.success;
    assert !incompleteSettingsEvaluation.success;
    verifyGeneratedFiles "browser-home-manager-module" homeDefaultFile homeNamedFile;

  version = browserPackage.passthru.tests.version;
}
// lib.optionalAttrs pkgs.stdenv.hostPlatform.isLinux (
  let
    nixos = lib.nixosSystem {
      inherit (pkgs.stdenv.hostPlatform) system;
      modules = [
        nixosModule
        moduleConfiguration
        { system.stateVersion = "25.11"; }
      ];
    };

    defaultFile = nixos.config.environment.etc."browser/browser.toml".source;
    namedFile = nixos.config.environment.etc."browser/second.toml".source;
  in
  {
    module-nixos =
      assert lib.elem browserPackage nixos.config.environment.systemPackages;
      assert nixos.config.programs.browser.configFile == defaultFile;
      assert nixos.config.programs.browser.configFiles.second == namedFile;
      verifyGeneratedFiles "browser-nixos-module" defaultFile namedFile;

    nixos-vm = import ./tests/nixos-vm.nix {
      inherit browserPackage nixosModule pkgs;
    };
  }
)
// lib.optionalAttrs pkgs.stdenv.hostPlatform.isDarwin (
  let
    darwin = nixDarwin.lib.darwinSystem {
      inherit pkgs;
      modules = [
        darwinModule
        moduleConfiguration
        { system.stateVersion = 6; }
      ];
    };

    defaultFile = darwin.config.environment.etc."browser/browser.toml".source;
    namedFile = darwin.config.environment.etc."browser/second.toml".source;
  in
  {
    module-nix-darwin =
      assert lib.elem browserPackage darwin.config.environment.systemPackages;
      assert darwin.config.programs.browser.configFile == defaultFile;
      assert darwin.config.programs.browser.configFiles.second == namedFile;
      verifyGeneratedFiles "browser-nix-darwin-module" defaultFile namedFile;
  }
)
