{
  config,
  lib,
  options,
  pkgs,
  ...
}:

let
  inherit (lib) types;

  cfg = config.programs.tweezle;
  tomlFormat = pkgs.formats.toml { };
  tweezleLib = import ../lib.nix { inherit lib; };
  providers = import ../providers { inherit lib; };
  settingsType = import ./settings.nix {
    inherit lib tomlFormat;
  };
  packageSet.tweezle = pkgs.tweezle or (pkgs.callPackage ../package.nix { });

  effectiveConfigurations =
    lib.attrsets.optionalAttrs (cfg.settings != null) {
      tweezle = cfg.settings;
    }
    // cfg.configurations;

  uncheckedGeneratedFiles = lib.attrsets.mapAttrs (
    name: settings:
    tweezleLib.generateConfig {
      inherit pkgs settings;
      name = "${name}.toml";
    }
  ) effectiveConfigurations;

  generatedFiles = lib.attrsets.mapAttrs (
    name: source:
    if cfg.checkConfig then
      pkgs.runCommandLocal "${name}.toml" { } ''
        ${lib.strings.escapeShellArg (lib.meta.getExe cfg.package)} config validate ${lib.strings.escapeShellArg source}
        cp ${lib.strings.escapeShellArg source} "$out"
      ''
    else
      source
  ) uncheckedGeneratedFiles;

  policyConfigurations = lib.attrsets.filterAttrs (
    _: settings: providers.hasPolicyIntent settings
  ) effectiveConfigurations;

  policyGroups = lib.lists.groupBy (
    name:
    let
      settings = policyConfigurations.${name};
      chromium = lib.trivial.defaultTo { } (settings.chromium or null);
    in
    if (settings.firefox or null) != null then
      "firefox"
    else if (chromium.chrome or null) != null then
      "chrome"
    else if (chromium.edge or null) != null then
      "edge"
    else
      "chromium"
  ) (builtins.attrNames policyConfigurations);

  managedPolicyFiles = lib.attrsets.mapAttrs (
    engine: names:
    pkgs.runCommandLocal "tweezle-${engine}-managed-policy.json" { } ''
      ${lib.strings.escapeShellArg (lib.meta.getExe packageSet.tweezle)} policy render \
        ${lib.strings.escapeShellArgs (map (name: generatedFiles.${name}) names)} \
        --output "$out"
    ''
  ) policyGroups;

  managedPolicyFile =
    if builtins.length (lib.attrsets.attrValues managedPolicyFiles) == 1 then
      builtins.head (lib.attrsets.attrValues managedPolicyFiles)
    else
      null;

  validConfigurationName =
    name: name != "tweezle" && builtins.match "[A-Za-z0-9][A-Za-z0-9._-]*" name != null;
in
{
  options.programs.tweezle = {
    enable = lib.options.mkEnableOption "Tweezle, the browser configurator";

    package = lib.options.mkPackageOption packageSet "tweezle" {
      nullable = true;
      pkgsText = "pkgs";
      extraDescription = ''
        Set this to `null` to manage configuration files without installing the
        package.
      '';
    };

    activate = lib.options.mkOption {
      type = types.bool;
      default = cfg.package != null;
      defaultText = lib.options.literalExpression "config.programs.tweezle.package != null";
      description = ''
        Apply the default and every named configuration during activation,
        creating launchers and updating configured browser profiles. Close
        the affected browsers before switching so they cannot overwrite
        profile changes.

        Home Manager applies configurations after linking the generation and
        respects dry runs. NixOS applies them through user activation, on
        switches for running user sessions and when other users next log in.
        nix-darwin applies them as `system.primaryUser` during the switch.

        Set this to `false` to manage files and apply them manually. A non-null
        {option}`programs.tweezle.package` is required for activation.
      '';
    };

    checkConfig = lib.options.mkOption {
      type = types.bool;
      default =
        cfg.package != null
        && lib.lists.all providers.configurationCanBeChecked (
          lib.attrsets.attrValues effectiveConfigurations
        );
      defaultText = lib.options.literalMD ''
        `true` when {option}`programs.tweezle.package` isn't `null` and every
        extension-settings document, runtime script, and JSON input is in
        the Nix store
      '';
      description = ''
        Whether to validate each generated TOML file with the configured
        `tweezle` package. This catches invalid cross-field settings while
        building the configuration.

        Validation defaults off when an extension-settings document, runtime
        script, or JSON input is outside the Nix store because that file isn't
        available in the build sandbox.
      '';
    };

    settings = lib.options.mkOption {
      type = types.nullOr settingsType;
      default = null;
      example = lib.options.literalExpression ''
        {
          chromium = {
            name = "Chromium";
            executable_name = "chromium-configured";

            linux = {
              app_dir = "''${pkgs.chromium}/bin";
              launcher_name = "chromium";
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
        Default configuration written to `tweezle/tweezle.toml`. When this is
        `null`, the module does not create a default configuration file.
        The configuration is applied during activation when
        {option}`programs.tweezle.activate` is enabled.

        Browser-family settings are grouped under `chromium` and `firefox`.
        Choose one browser family for this default configuration. Use
        {option}`programs.tweezle.configurations` for additional setups.
        Chromium includes its extension catalog, extension storage, and
        uBlock settings; the generated file uses the existing TOML tables.
        Known settings are typed and documented. Additional TOML keys are
        accepted within each browser family.
      '';
    };

    configurations = lib.options.mkOption {
      type = types.attrsWith {
        elemType = settingsType;
        placeholder = "configuration";
      };
      default = { };
      example = lib.options.literalExpression ''
        {
          personal.chromium = {
            name = "Chromium";
            executable_name = "chromium";
            linux = {
              app_dir = "''${pkgs.chromium}/bin";
              launcher_name = "chromium";
            };
          };

          work.firefox = {
            variant = "firefox";
            paths.linux.profile_dir = "''${config_home}/mozilla/firefox/REPLACE.default-release";
            preferences."browser.startup.page" = 3;
          };
        }
      '';
      description = ''
        Optional named browser configurations, separate from the default
        {option}`programs.tweezle.settings`. Each uses the same `chromium`
        or `firefox` settings structure. A configuration named `NAME`
        is written to `tweezle/NAME.toml` and is available as
        {option}`programs.tweezle.configFiles.NAME`. The name `tweezle` is
        reserved for the default {option}`programs.tweezle.settings` file.
        Every named configuration is applied during activation when
        {option}`programs.tweezle.activate` is enabled.
      '';
    };

    configFile = lib.options.mkOption {
      type = types.nullOr types.pathInStore;
      readOnly = true;
      description = ''
        Generated default `tweezle.toml`, or `null` when `settings` is not
        managed.
      '';
    };

    configFiles = lib.options.mkOption {
      type = types.attrsOf types.pathInStore;
      readOnly = true;
      description = ''
        Generated configuration files keyed by basename. The default
        configuration, when present, is keyed as `tweezle`.
      '';
    };

    managedPolicyFile = lib.options.mkOption {
      type = types.nullOr types.pathInStore;
      readOnly = true;
      description = ''
        Generated managed-policy JSON when exactly one policy store has
        policy intent, or `null` otherwise. Use managedPolicyFiles when
        managing multiple browser products or engines.
      '';
    };

    managedPolicyFiles = lib.options.mkOption {
      type = types.attrsOf types.pathInStore;
      readOnly = true;
      description = ''
        Generated managed-policy JSON keyed by policy store: `chrome`, `edge`,
        `chromium` for other Chromium providers, and `firefox`. Different
        products are rendered separately even when they share JSON syntax.
        Conflicts within a store are rejected.
      '';
    };

    fileSources = lib.options.mkOption {
      type = types.attrsOf types.pathInStore;
      default = { };
      internal = true;
      description = ''
        Generated files keyed by their path relative to the configuration
        directory. Platform modules install these under the XDG configuration
        directory or `/etc`; providers may add compatibility paths.
      '';
    };
  };

  config = {
    assertions = [
      {
        assertion = cfg.activate -> cfg.package != null;
        message = ''
          `programs.tweezle.activate` requires a non-null
          `programs.tweezle.package`.
        '';
      }
      {
        assertion = cfg.checkConfig -> cfg.package != null;
        message = ''
          `programs.tweezle.checkConfig` requires a non-null
          `programs.tweezle.package`.
        '';
      }
    ]
    ++ lib.attrsets.mapAttrsToList (name: _: {
      assertion = validConfigurationName name;
      message = ''
        `programs.tweezle.configurations` has invalid name
        ${builtins.toJSON name}. Names must contain only letters, numbers,
        dots, underscores, and hyphens, must begin with a letter or number,
        and may not be "tweezle".
      '';
    }) cfg.configurations;

    programs.tweezle = {
      configFile = lib.modules.mkDerivedConfig options.programs.tweezle.configFiles (
        files: files.tweezle or null
      );
      configFiles = generatedFiles;
      inherit managedPolicyFile managedPolicyFiles;
      fileSources = lib.modules.mkIf cfg.enable (
        lib.modules.mkMerge [
          (lib.attrsets.mapAttrs' (
            name: source: lib.attrsets.nameValuePair "tweezle/${name}.toml" source
          ) cfg.configFiles)
          (lib.attrsets.mapAttrs' (
            engine: source: lib.attrsets.nameValuePair "tweezle/managed-policy-${engine}.json" source
          ) cfg.managedPolicyFiles)
          (lib.modules.mkIf (cfg.managedPolicyFile != null) {
            "tweezle/managed-policy.json" = cfg.managedPolicyFile;
          })
        ]
      );
    };
  };
}
