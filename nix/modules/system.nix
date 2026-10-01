{
  config,
  lib,
  ...
}:

let
  cfg = config.programs.browser;
in
{
  imports = [ ./options.nix ] ++ (import ../providers { inherit lib; }).systemModules;

  config = lib.mkIf cfg.enable {
    environment.systemPackages = lib.optional (cfg.package != null) cfg.package;

    environment.etc = lib.mkMerge [
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
