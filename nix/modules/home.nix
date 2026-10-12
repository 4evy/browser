{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.programs.tweezle;
in
{
  _class = "homeManager";

  meta.maintainers = [ lib.maintainers._4evy ];

  imports = [ ./options.nix ] ++ (import ../providers { inherit lib; }).homeManagerModules;

  config = lib.modules.mkIf cfg.enable {
    home.packages = lib.lists.optional (cfg.package != null) cfg.package;

    xdg.configFile = lib.attrsets.mapAttrs (_: source: { inherit source; }) cfg.fileSources;

    home.activation.tweezle = lib.modules.mkIf (cfg.activate && cfg.configFiles != { }) (
      lib.hm.dag.entryAfter [ "linkGeneration" ] ''
        (
          export HOME=${lib.strings.escapeShellArg config.home.homeDirectory}
          export XDG_CONFIG_HOME=${lib.strings.escapeShellArg config.xdg.configHome}
          export XDG_DATA_HOME=${lib.strings.escapeShellArg config.xdg.dataHome}
          export XDG_CACHE_HOME=${lib.strings.escapeShellArg config.xdg.cacheHome}
          export XDG_STATE_HOME=${lib.strings.escapeShellArg config.xdg.stateHome}
          export PATH="${config.home.path}/bin:/run/current-system/sw/bin:/usr/bin:/bin:$PATH"
          cd "$HOME"
          ${lib.strings.concatStringsSep "\n" (
            lib.attrsets.mapAttrsToList (
              name: _:
              "run "
              + lib.strings.escapeShellArgs [
                (lib.meta.getExe cfg.package)
                "apply"
                "${config.xdg.configHome}/tweezle/${name}.toml"
                "--platform"
                (if pkgs.stdenv.hostPlatform.isDarwin then "macos" else "linux")
              ]
            ) cfg.configFiles
          )}
        )
      ''
    );
  };
}
