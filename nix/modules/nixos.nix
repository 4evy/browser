{
  config,
  lib,
  ...
}:

let
  cfg = config.programs.tweezle;
in

{
  _class = "nixos";

  meta.maintainers = [ lib.maintainers._4evy ];

  imports = [ ./system.nix ] ++ (import ../providers { inherit lib; }).nixosModules;

  config = lib.modules.mkIf (cfg.enable && cfg.activate && cfg.configFiles != { }) {
    system.userActivationScripts.tweezle.text = ''
      (
        export PATH="${config.system.path}/bin:$HOME/.nix-profile/bin:/etc/profiles/per-user/$USER/bin:$PATH"
        cd "$HOME"
        ${lib.strings.concatStringsSep "\n" (
          lib.attrsets.mapAttrsToList (
            name: _:
            lib.strings.escapeShellArgs [
              (lib.meta.getExe cfg.package)
              "apply"
              "/etc/tweezle/${name}.toml"
              "--platform"
              "linux"
            ]
            + " || exit $?"
          ) cfg.configFiles
        )}
      )
    '';
  };
}
