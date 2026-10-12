{
  config,
  lib,
  ...
}:

let
  cfg = config.programs.tweezle;
in
{
  imports = [ ./options.nix ] ++ (import ../providers { inherit lib; }).systemModules;

  config = lib.modules.mkIf cfg.enable {
    environment.systemPackages = lib.lists.optional (cfg.package != null) cfg.package;

    environment.etc = lib.attrsets.mapAttrs (_: source: { inherit source; }) cfg.fileSources;
  };
}
