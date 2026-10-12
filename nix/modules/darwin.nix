{
  config,
  lib,
  ...
}:

let
  cfg = config.programs.tweezle;
in
{
  _class = "darwin";

  imports = [ ./system.nix ];

  config = lib.modules.mkIf (cfg.enable && cfg.activate && cfg.configFiles != { }) {
    system.requiresPrimaryUser = [ "programs.tweezle.activate" ];

    system.activationScripts.postActivation.text =
      lib.modules.mkIf (config.system.primaryUser != null)
        (
          lib.modules.mkAfter (
            lib.strings.concatStringsSep "\n" (
              lib.attrsets.mapAttrsToList (
                name: _:
                "/usr/bin/sudo -H --user=${lib.strings.escapeShellArg config.system.primaryUser} -- "
                + lib.strings.escapeShellArgs [
                  "/usr/bin/env"
                  "-i"
                  "HOME=${config.system.primaryUserHome}"
                  "USER=${config.system.primaryUser}"
                  "LOGNAME=${config.system.primaryUser}"
                  "PATH=${config.system.path}/bin:/usr/bin:/bin:/usr/sbin:/sbin"
                  (lib.meta.getExe cfg.package)
                  "apply"
                  "/etc/tweezle/${name}.toml"
                  "--platform"
                  "macos"
                ]
              ) cfg.configFiles
            )
          )
        );
  };
}
