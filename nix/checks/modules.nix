{
  tweezlePackage,
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

  expectedDefaultSettings = pkgs.writeText "tweezle-expected-settings.json" (
    builtins.toJSON {
      browser = removeAttrs sampleSettings.chromium [
        "extensions"
        "extension_settings"
      ];
      inherit (sampleSettings.chromium) extensions extension_settings;
    }
  );

  expectedNamedSettings = pkgs.writeText "tweezle-expected-named-settings.json" (
    builtins.toJSON {
      browser = namedSettings.chromium // {
        paths = { };
      };
    }
  );

  expectedManagedPolicy = pkgs.writeText "tweezle-expected-managed-policy.json" (
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
      DefaultBrowserSettingEnabled = false;
      MetricsReportingEnabled = false;
      PsstEnabled = false;
      TorDisabled = true;
    }
  );

  verificationPackage = tweezlePackage.overrideAttrs {
    pname = "tweezle-module-verification";
    subPackages = [
      "nix/checks/modules/policies"
      "nix/checks/modules/verify-files"
    ];
    doCheck = false;
    postInstall = "";
    postFixup = "";
    ldflags = [ ];
  };

  verifyGeneratedFiles =
    name: defaultFile: namedFile: managedPolicyFile:
    pkgs.runCommandLocal name {
      inherit
        defaultFile
        namedFile
        expectedDefaultSettings
        expectedNamedSettings
        managedPolicyFile
        expectedManagedPolicy
        ;
      verificationExecutable = "${verificationPackage}/bin/verify-files";
      tweezleExecutable = lib.meta.getExe tweezlePackage;
    } (builtins.readFile ./modules/modules.sh);

  home = mkHome moduleConfiguration;

  productConfiguration.programs.tweezle = {
    enable = true;
    package = tweezlePackage;
    configurations = {
      chrome.chromium = {
        executable_name = "chrome";
        chrome.sync = false;
      };
      edge.chromium = {
        executable_name = "edge";
        edge.tracking_prevention = "strict";
      };
      firefox.firefox.policies.DisableTelemetry = true;
    };
  };
  productHome = mkHome productConfiguration;
  expectedProductPolicies = pkgs.writeText "tweezle-product-policies.json" (
    builtins.toJSON {
      chrome.SyncDisabled = true;
      edge.TrackingPrevention = 3;
      firefox.policies.DisableTelemetry = true;
    }
  );
  verifyProductPolicies =
    name: files:
    pkgs.runCommandLocal name {
      policyPaths = pkgs.writeText "tweezle-product-policy-paths.json" (builtins.toJSON files);
      inherit expectedProductPolicies;
      verificationExecutable = "${verificationPackage}/bin/policies";
    } (builtins.readFile ./modules/policies.sh);
  homeDefaultFile = home.config.xdg.configFile."tweezle/tweezle.toml".source;
  homeNamedFile = home.config.xdg.configFile."tweezle/second.toml".source;
  homeManagedPolicyFile = home.config.xdg.configFile."tweezle/brave-managed-policy.json".source;

  homeDisabled = mkHome {
    programs.tweezle = {
      package = tweezlePackage;
      settings = namedSettings;
    };
  };

  homeWithoutSettings = mkHome {
    programs.tweezle = {
      enable = true;
      package = null;
    };
  };

  homeWithDefaultPackage = mkHome {
    programs.tweezle.enable = true;
  };

  homeConfigOnly = mkHome {
    programs.tweezle = {
      enable = true;
      package = null;
      settings = namedSettings;
    };
  };

  homeWithRelativeExtensionSettings = mkHome {
    programs.tweezle = {
      enable = true;
      package = tweezlePackage;
      settings = lib.attrsets.recursiveUpdate namedSettings {
        chromium.extension_settings.files = [ "relative-settings.json" ];
      };
    };
  };

  homeWithInvalidName = mkHome {
    programs.tweezle = {
      enable = true;
      package = null;
      configurations."../escape" = namedSettings;
    };
  };

  invalidNameEvaluation = builtins.tryEval homeWithInvalidName.config.home.username;

  homeWithInvalidCheckConfig = mkHome {
    programs.tweezle = {
      enable = true;
      checkConfig = true;
      package = null;
      settings = namedSettings;
    };
  };

  invalidCheckConfigEvaluation = builtins.tryEval homeWithInvalidCheckConfig.config.home.username;

  homeWithIncompleteSettings = mkHome {
    programs.tweezle = {
      enable = true;
      package = null;
      settings.chromium.name = "Missing executable name";
    };
  };

  incompleteSettingsEvaluation = builtins.tryEval homeWithIncompleteSettings.config.programs.tweezle.configFile;

  wrongClass =
    builtins.tryEval
      (lib.modules.evalModules {
        class = "nixos";
        specialArgs = { inherit pkgs; };
        modules = [ darwinModule ];
      }).config.programs.tweezle.enable;

  declarationRoot = toString ../..;
  moduleOptions = {
    programs.tweezle = home.options.programs.tweezle;
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
              lib.strings.removePrefix "${declarationRoot}/" (toString declaration)
            );
          in
          {
            name = relative;
            url = "https://github.com/4evy/tweezle/blob/master/${relative}";
          }
        ) option.declarations;
      };
  };
in
{
  module-classes =
    assert !wrongClass.success;
    assert (import ../../addons/brave/nix/policy.nix { inherit lib; }).hasPolicyIntent {
      chromium.brave.behavior.show_default_browser_prompt = false;
    };
    assert
      !(import ../../addons/brave/nix/policy.nix { inherit lib; }).hasPolicyIntent {
        chromium.brave.behavior.show_default_browser_prompt = true;
      };
    assert
      !(import ../../addons/brave/nix/policy.nix { inherit lib; }).hasPolicyIntent {
        chromium.brave.behavior = null;
      };
    pkgs.runCommandLocal "tweezle-module-classes" { } "touch $out";

  module-home-manager =
    assert lib.lists.elem tweezlePackage home.config.home.packages;
    assert home.config.programs.tweezle.configFile == homeDefaultFile;
    assert home.config.programs.tweezle.configFiles.second == homeNamedFile;
    assert home.config.programs.tweezle.managedPolicyFile == homeManagedPolicyFile;
    assert !lib.lists.elem tweezlePackage homeDisabled.config.home.packages;
    assert !builtins.hasAttr "tweezle/tweezle.toml" homeDisabled.config.xdg.configFile;
    assert homeWithoutSettings.config.programs.tweezle.configFile == null;
    assert homeWithoutSettings.config.programs.tweezle.configFiles == { };
    assert homeWithoutSettings.config.programs.tweezle.managedPolicyFile == null;
    assert !builtins.hasAttr "tweezle/tweezle.toml" homeWithoutSettings.config.xdg.configFile;
    assert lib.attrsets.isDerivation homeWithDefaultPackage.config.programs.tweezle.package;
    assert lib.lists.elem homeWithDefaultPackage.config.programs.tweezle.package
      homeWithDefaultPackage.config.home.packages;
    assert !lib.lists.elem tweezlePackage homeConfigOnly.config.home.packages;
    assert builtins.hasAttr "tweezle/tweezle.toml" homeConfigOnly.config.xdg.configFile;
    assert home.config.programs.tweezle.checkConfig;
    assert !homeConfigOnly.config.programs.tweezle.checkConfig;
    assert !homeWithRelativeExtensionSettings.config.programs.tweezle.checkConfig;
    assert !invalidCheckConfigEvaluation.success;
    assert !invalidNameEvaluation.success;
    assert !incompleteSettingsEvaluation.success;
    verifyGeneratedFiles "tweezle-home-manager-module" homeDefaultFile homeNamedFile
      homeManagedPolicyFile;

  module-options = moduleOptionsDoc.optionsJSON;

  module-product-policies =
    assert productHome.config.programs.tweezle.managedPolicyFile == null;
    assert
      builtins.attrNames productHome.config.programs.tweezle.managedPolicyFiles == [
        "chrome"
        "edge"
        "firefox"
      ];
    verifyProductPolicies "tweezle-home-product-policies" productHome.config.programs.tweezle.managedPolicyFiles;
}
// lib.attrsets.optionalAttrs pkgs.stdenv.hostPlatform.isLinux (
  let
    nixos = lib.nixosSystem {
      inherit (pkgs.stdenv.hostPlatform) system;
      modules = [
        nixosModule
        moduleConfiguration
        { system.stateVersion = "25.11"; }
      ];
    };

    defaultFile = nixos.config.environment.etc."tweezle/tweezle.toml".source;
    namedFile = nixos.config.environment.etc."tweezle/second.toml".source;
    managedPolicyFile = nixos.config.environment.etc."brave/policies/managed/tweezle.json".source;
    productNixos = lib.nixosSystem {
      inherit (pkgs.stdenv.hostPlatform) system;
      modules = [
        nixosModule
        productConfiguration
        { system.stateVersion = "25.11"; }
      ];
    };
  in
  {
    module-nixos =
      assert lib.lists.elem tweezlePackage nixos.config.environment.systemPackages;
      assert nixos.config.programs.tweezle.configFile == defaultFile;
      assert nixos.config.programs.tweezle.configFiles.second == namedFile;
      assert nixos.config.environment.etc."tweezle/brave-managed-policy.json".source == managedPolicyFile;
      verifyGeneratedFiles "tweezle-nixos-module" defaultFile namedFile managedPolicyFile;

    nixos-vm = import ../tests/nixos.nix {
      inherit tweezlePackage nixosModule pkgs;
    };

    module-nixos-product-policies =
      assert productNixos.config.programs.tweezle.managedPolicyFile == null;
      verifyProductPolicies "tweezle-nixos-product-policies" {
        chrome = productNixos.config.environment.etc."opt/chrome/policies/managed/tweezle.json".source;
        edge = productNixos.config.environment.etc."opt/edge/policies/managed/tweezle.json".source;
        firefox = productNixos.config.environment.etc."tweezle/managed-policy-firefox.json".source;
      };
  }
)
// lib.attrsets.optionalAttrs pkgs.stdenv.hostPlatform.isDarwin (
  let
    darwin = nixDarwin.lib.darwinSystem {
      inherit pkgs;
      modules = [
        darwinModule
        moduleConfiguration
        { system.stateVersion = 6; }
      ];
    };

    defaultFile = darwin.config.environment.etc."tweezle/tweezle.toml".source;
    namedFile = darwin.config.environment.etc."tweezle/second.toml".source;
    managedPolicyFile = darwin.config.environment.etc."tweezle/brave-managed-policy.json".source;
  in
  {
    module-nix-darwin =
      assert lib.lists.elem tweezlePackage darwin.config.environment.systemPackages;
      assert darwin.config.programs.tweezle.configFile == defaultFile;
      assert darwin.config.programs.tweezle.configFiles.second == namedFile;
      verifyGeneratedFiles "tweezle-nix-darwin-module" defaultFile namedFile managedPolicyFile;
  }
)
