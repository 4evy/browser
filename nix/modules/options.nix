{
  config,
  lib,
  options,
  pkgs,
  ...
}:

let
  inherit (lib)
    attrVals
    attrValues
    escapeShellArgs
    filterAttrs
    literalExpression
    literalMD
    mapAttrs
    mapAttrsToList
    mkDerivedConfig
    mkEnableOption
    mkOption
    optionalAttrs
    types
    ;

  cfg = config.programs.browser;
  tomlFormat = pkgs.formats.toml { };
  browserLib = import ../lib.nix { inherit lib; };
  bravePolicyLib = import ./brave-policy.nix { inherit lib; };
  settingsType = import ./settings.nix {
    inherit lib tomlFormat;
  };
  packageSet.browser = pkgs.browser or (pkgs.callPackage ../package.nix { });

  effectiveConfigurations =
    optionalAttrs (cfg.settings != null) {
      browser = cfg.settings;
    }
    // cfg.configurations;

  uncheckedGeneratedFiles = mapAttrs (
    name: settings:
    browserLib.generateConfig {
      inherit pkgs settings;
      name = "${name}.toml";
    }
  ) effectiveConfigurations;

  configurationCanBeChecked =
    settings:
    let
      extensionSettings = settings.extension_settings or null;
      files = if extensionSettings == null then [ ] else extensionSettings.files or [ ];
    in
    lib.all types.pathInStore.check files;

  generatedFiles = mapAttrs (
    name: source:
    if cfg.checkConfig then
      pkgs.runCommandLocal "${name}.toml" { } ''
        ${lib.getExe cfg.package} config validate ${source}
        cp ${source} "$out"
      ''
    else
      source
  ) uncheckedGeneratedFiles;

  policyConfigurations = filterAttrs (
    _: settings: bravePolicyLib.hasPolicyIntent settings
  ) effectiveConfigurations;

  managedPolicyFile =
    if policyConfigurations == { } then
      null
    else
      pkgs.runCommandLocal "browser-brave-managed-policy.json" { } ''
        ${lib.getExe packageSet.browser} policy render \
          ${escapeShellArgs (attrVals (builtins.attrNames policyConfigurations) generatedFiles)} \
          --output "$out"
      '';

  validConfigurationName =
    name: name != "browser" && builtins.match "[A-Za-z0-9][A-Za-z0-9._-]*" name != null;
in
{
  options.programs.browser = {
    enable = mkEnableOption "the browser configurator";

    package = lib.mkPackageOption packageSet "browser" {
      nullable = true;
      pkgsText = "pkgs";
      extraDescription = ''
        Set this to `null` to manage configuration files without installing the
        package.
      '';
    };

    checkConfig = mkOption {
      type = types.bool;
      default =
        cfg.package != null && lib.all configurationCanBeChecked (attrValues effectiveConfigurations);
      defaultText = literalMD ''
        `true` when {option}`programs.browser.package` isn't `null` and every
        extension-settings file is in the Nix store
      '';
      description = ''
        Whether to validate each generated TOML file with the configured
        `browser` package. This catches invalid cross-field settings while
        building the configuration.

        Validation defaults off when an extension-settings file is outside the
        Nix store because that file isn't available in the build sandbox.
      '';
    };

    settings = mkOption {
      type = types.nullOr settingsType;
      default = null;
      example = literalExpression ''
        {
          browser = {
            name = "Chromium";
            executable_name = "chromium";
            flags_file = "chromium-flags.conf";

            linux = {
              desktop_id = "chromium";
              launcher_name = "chromium";
              desktop_name = "chromium.desktop";
              desktop_exec = "chromium";
            };

            paths.linux = {
              profile_dir = "''${config_home}/chromium/Default";
              external_extension_dirs = [
                "''${config_home}/chromium/External Extensions"
              ];
            };
          };
        }
      '';
      description = ''
        Default configuration written to `browser/browser.toml`. When this is
        `null`, the module does not create a default configuration file.

        Known settings are typed and documented. Additional TOML-compatible
        keys are accepted for forward compatibility.
      '';
    };

    configurations = mkOption {
      type = types.attrsWith {
        elemType = settingsType;
        placeholder = "configuration";
      };
      default = { };
      example = literalExpression ''
        {
          chromium.browser = {
            name = "Chromium";
            executable_name = "chromium";
          };

          brave.browser = {
            name = "Brave";
            executable_name = "brave";
          };
        }
      '';
      description = ''
        Additional named browser configurations. A configuration named `NAME`
        is written to `browser/NAME.toml` and is available as
        {option}`programs.browser.configFiles.NAME`. The name `browser` is
        reserved for the default {option}`programs.browser.settings` file.
      '';
    };

    configFile = mkOption {
      type = types.nullOr types.pathInStore;
      readOnly = true;
      description = ''
        Generated default `browser.toml`, or `null` when `settings` is not
        managed.
      '';
    };

    configFiles = mkOption {
      type = types.attrsOf types.pathInStore;
      readOnly = true;
      description = ''
        Generated configuration files keyed by basename. The default
        configuration, when present, is keyed as `browser`.
      '';
    };

    managedPolicyFile = mkOption {
      type = types.nullOr types.pathInStore;
      readOnly = true;
      description = ''
        Generated Chromium managed-policy JSON for all Brave configurations,
        or `null` when no Brave managed policy is configured.
      '';
    };
  };

  config = {
    assertions = [
      {
        assertion = cfg.checkConfig -> cfg.package != null;
        message = ''
          `programs.browser.checkConfig` requires a non-null
          `programs.browser.package`.
        '';
      }
    ]
    ++ mapAttrsToList (name: _: {
      assertion = validConfigurationName name;
      message = ''
        `programs.browser.configurations` has invalid name
        ${builtins.toJSON name}. Names must contain only letters, numbers,
        dots, underscores, and hyphens, must begin with a letter or number,
        and may not be "browser".
      '';
    }) cfg.configurations;

    programs.browser = {
      configFile = mkDerivedConfig options.programs.browser.configFiles (files: files.browser or null);
      configFiles = generatedFiles;
      inherit managedPolicyFile;
    };
  };
}
