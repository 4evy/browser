{
  config,
  lib,
  ...
}:

let
  cfg = config.programs.browser;
in
{
  _class = "homeManager";

  imports = [ ./options.nix ] ++ (import ../providers { inherit lib; }).homeManagerModules;

  config = lib.mkIf cfg.enable {
    home.packages = lib.optional (cfg.package != null) cfg.package;

    xdg.configFile = lib.mkMerge [
      (lib.mapAttrs' (
        name: source:
        lib.nameValuePair "browser/${name}.toml" {
          inherit source;
        }
      ) cfg.configFiles)
      (lib.mkIf (cfg.managedPolicyFile != null) {
        "browser/managed-policy.json".source = cfg.managedPolicyFile;
      })
    ];
  };
}
