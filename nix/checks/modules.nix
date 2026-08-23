{
  browserPackage,
  darwinModule,
  fixtures,
  homeManager,
  homeManagerModule,
  lib,
  nixDarwin,
  nixosModule,
  pkgs,
}:

let
  inherit (fixtures)
    homeBase
    moduleConfiguration
    namedSettings
    sampleSettings
    ;

  homeModuleBase = homeManager.lib.homeManagerConfiguration {
    inherit pkgs;
    modules = [
      homeManagerModule
      homeBase
    ];
  };

  mkHome =
    module:
    homeModuleBase.extendModules {
      modules = [ module ];
    };

  expectedDefaultSettings = pkgs.writeText "browser-expected-settings.json" (
    builtins.toJSON sampleSettings
  );

  expectedNamedSettings = pkgs.writeText "browser-expected-named-settings.json" (
    builtins.toJSON (lib.recursiveUpdate namedSettings { browser.paths = { }; })
  );

  expectedManagedPolicy = pkgs.writeText "browser-expected-managed-policy.json" (
    builtins.toJSON {
      BraveAIChatEnabled = false;
      BraveLocalAIEnabled = false;
      BraveNewsDisabled = true;
      BraveP3AEnabled = false;
      BravePlaylistEnabled = false;
      BraveRewardsDisabled = true;
      BraveSpeedreaderEnabled = false;
      BraveStatsPingEnabled = false;
      BraveTalkDisabled = true;
      BraveVPNDisabled = true;
      BraveWalletDisabled = true;
      BraveWaybackMachineEnabled = false;
      BraveWebDiscoveryEnabled = false;
      BrowserSignin = 0;
      EmailAliasesEnabled = false;
      IPFSEnabled = false;
      MetricsReportingEnabled = false;
      PsstEnabled = false;
      TorDisabled = true;
    }
  );

  verifyGeneratedFiles =
    name: defaultFile: namedFile: managedPolicyFile:
    pkgs.runCommandLocal name
      {
        nativeBuildInputs = [ pkgs.python3 ];
      }
      ''
        python3 - \
          ${defaultFile} \
          ${namedFile} \
          ${expectedDefaultSettings} \
          ${expectedNamedSettings} \
          ${managedPolicyFile} \
          ${expectedManagedPolicy} <<'PY'
        import json
        import sys
        import tomllib

        def load_toml(path):
            with open(path, "rb") as source:
                return tomllib.load(source)

        def load_json(path):
            with open(path, encoding="utf-8") as source:
                return json.load(source)

        def check(actual_path, expected_path):
            actual = load_toml(actual_path)
            expected = load_json(expected_path)
            if actual != expected:
                print(f"generated configuration differs: {actual_path}", file=sys.stderr)
                print("expected:", json.dumps(expected, indent=2, sort_keys=True), file=sys.stderr)
                print("actual:", json.dumps(actual, indent=2, sort_keys=True), file=sys.stderr)
                raise SystemExit(1)

        def check_json(actual_path, expected_path):
            actual = load_json(actual_path)
            expected = load_json(expected_path)
            if actual != expected:
                print(f"generated JSON differs: {actual_path}", file=sys.stderr)
                print("expected:", json.dumps(expected, indent=2, sort_keys=True), file=sys.stderr)
                print("actual:", json.dumps(actual, indent=2, sort_keys=True), file=sys.stderr)
                raise SystemExit(1)

        check(sys.argv[1], sys.argv[3])
        check(sys.argv[2], sys.argv[4])
        check_json(sys.argv[5], sys.argv[6])
        PY

        for configFile in ${defaultFile} ${namedFile}; do
          profile="$TMPDIR/profile-$(basename "$configFile" .toml)"
          ${lib.getExe browserPackage} profile apply "$configFile" \
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
  homeManagedPolicyFile = home.config.xdg.configFile."browser/brave-managed-policy.json".source;

  homeDisabled = mkHome {
    programs.browser = {
      package = browserPackage;
      settings = namedSettings;
    };
  };

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

  homeWithRelativeExtensionSettings = mkHome {
    programs.browser = {
      enable = true;
      package = browserPackage;
      settings = lib.recursiveUpdate namedSettings {
        extension_settings.files = [ "relative-settings.json" ];
      };
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

  homeWithInvalidCheckConfig = mkHome {
    programs.browser = {
      enable = true;
      checkConfig = true;
      package = null;
      settings = namedSettings;
    };
  };

  invalidCheckConfigEvaluation = builtins.tryEval homeWithInvalidCheckConfig.config.home.username;

  homeWithIncompleteSettings = mkHome {
    programs.browser = {
      enable = true;
      package = null;
      settings.browser.name = "Missing executable name";
    };
  };

  incompleteSettingsEvaluation = builtins.tryEval homeWithIncompleteSettings.config.programs.browser.configFile;

  wrongClass =
    builtins.tryEval
      (lib.evalModules {
        class = "nixos";
        specialArgs = { inherit pkgs; };
        modules = [ darwinModule ];
      }).config.programs.browser.enable;

  declarationRoot = toString ../..;
  moduleOptions = {
    programs.browser = home.options.programs.browser;
  };
  moduleOptionsDoc = pkgs.nixosOptionsDoc {
    options = moduleOptions;
    documentType = "none";
    transformOptions =
      option:
      option
      // {
        declarations = map (
          declaration:
          let
            relative = builtins.unsafeDiscardStringContext (
              lib.removePrefix "${declarationRoot}/" (toString declaration)
            );
          in
          {
            name = relative;
            url = "https://github.com/4evy/browser/blob/master/${relative}";
          }
        ) option.declarations;
      };
  };
in
{
  module-classes =
    assert !wrongClass.success;
    pkgs.runCommandLocal "browser-module-classes" { } "touch $out";

  module-home-manager =
    assert lib.elem browserPackage home.config.home.packages;
    assert home.config.programs.browser.configFile == homeDefaultFile;
    assert home.config.programs.browser.configFiles.second == homeNamedFile;
    assert home.config.programs.browser.managedPolicyFile == homeManagedPolicyFile;
    assert !lib.elem browserPackage homeDisabled.config.home.packages;
    assert !builtins.hasAttr "browser/browser.toml" homeDisabled.config.xdg.configFile;
    assert homeWithoutSettings.config.programs.browser.configFile == null;
    assert homeWithoutSettings.config.programs.browser.configFiles == { };
    assert homeWithoutSettings.config.programs.browser.managedPolicyFile == null;
    assert !builtins.hasAttr "browser/browser.toml" homeWithoutSettings.config.xdg.configFile;
    assert lib.isDerivation homeWithDefaultPackage.config.programs.browser.package;
    assert lib.elem homeWithDefaultPackage.config.programs.browser.package
      homeWithDefaultPackage.config.home.packages;
    assert !lib.elem browserPackage homeConfigOnly.config.home.packages;
    assert builtins.hasAttr "browser/browser.toml" homeConfigOnly.config.xdg.configFile;
    assert home.config.programs.browser.checkConfig;
    assert !homeConfigOnly.config.programs.browser.checkConfig;
    assert !homeWithRelativeExtensionSettings.config.programs.browser.checkConfig;
    assert !invalidCheckConfigEvaluation.success;
    assert !invalidNameEvaluation.success;
    assert !incompleteSettingsEvaluation.success;
    verifyGeneratedFiles "browser-home-manager-module" homeDefaultFile homeNamedFile
      homeManagedPolicyFile;

  module-options = moduleOptionsDoc.optionsJSON;
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
    managedPolicyFile = nixos.config.environment.etc."brave/policies/managed/browser.json".source;
  in
  {
    module-nixos =
      assert lib.elem browserPackage nixos.config.environment.systemPackages;
      assert nixos.config.programs.browser.configFile == defaultFile;
      assert nixos.config.programs.browser.configFiles.second == namedFile;
      assert nixos.config.environment.etc."browser/brave-managed-policy.json".source == managedPolicyFile;
      verifyGeneratedFiles "browser-nixos-module" defaultFile namedFile managedPolicyFile;

    nixos-vm = import ../tests/nixos-vm.nix {
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
    managedPolicyFile = darwin.config.environment.etc."browser/brave-managed-policy.json".source;
  in
  {
    module-nix-darwin =
      assert lib.elem browserPackage darwin.config.environment.systemPackages;
      assert darwin.config.programs.browser.configFile == defaultFile;
      assert darwin.config.programs.browser.configFiles.second == namedFile;
      verifyGeneratedFiles "browser-nix-darwin-module" defaultFile namedFile managedPolicyFile;
  }
)
