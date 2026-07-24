{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.programs.browser;
  tomlFormat = pkgs.formats.toml { };
  browserLib = import ../lib.nix { inherit lib; };
  settingsType = import ./settings.nix {
    inherit lib tomlFormat;
  };

  configurations =
    lib.optionalAttrs (cfg.settings != null) {
      browser = cfg.settings;
    }
    // cfg.configurations;

  generatedFiles = lib.mapAttrs (
    name: settings:
    browserLib.generateConfig {
      inherit pkgs settings;
      name = "${name}.toml";
    }
  ) configurations;

  validConfigurationName =
    name:
    name != "browser"
    && name != "."
    && name != ".."
    && builtins.match "[A-Za-z0-9][A-Za-z0-9._-]*" name != null;
in
{
  _file = ./options.nix;
  _class = null;

  options.programs.browser = {
    enable = lib.mkEnableOption "the browser configurator";

    package =
      lib.mkPackageOption pkgs "browser" {
        nullable = true;
        extraDescription = "Set this to `null` to manage configuration files without installing the package.";
      }
      // {
        default = pkgs.browser or (pkgs.callPackage ../../package.nix { });
      };

    settings = lib.mkOption {
      type = lib.types.nullOr settingsType;
      default = null;
      example = lib.literalExpression ''
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

    configurations = lib.mkOption {
      type = lib.types.attrsOf settingsType;
      default = { };
      example = lib.literalExpression ''
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
        `programs.browser.configFiles.NAME`. The name `browser` is reserved for
        the default `settings` file.
      '';
    };

    configFile = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      readOnly = true;
      description = ''
        Generated default `browser.toml`, or `null` when `settings` is not
        managed.
      '';
    };

    configFiles = lib.mkOption {
      type = lib.types.attrsOf lib.types.path;
      readOnly = true;
      description = ''
        Generated configuration files keyed by basename. The default
        configuration, when present, is keyed as `browser`.
      '';
    };
  };

  config = {
    assertions = lib.mapAttrsToList (name: _: {
      assertion = validConfigurationName name;
      message = ''
        programs.browser.configurations has invalid name ${builtins.toJSON name}.
        Names must contain only letters, numbers, dots, underscores, and
        hyphens, must begin with a letter or number, and may not be "browser".
      '';
    }) cfg.configurations;

    programs.browser = {
      configFile = generatedFiles.browser or null;
      configFiles = generatedFiles;
    };
  };
}
