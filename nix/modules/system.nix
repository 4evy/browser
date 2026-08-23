{
  config,
  lib,
  ...
}:

let
  cfg = config.programs.browser;
in
{
  imports = [ ./options.nix ];

  config = lib.mkIf cfg.enable {
    environment.systemPackages = lib.optional (cfg.package != null) cfg.package;

    environment.etc =
      lib.mapAttrs' (
        name: source:
        lib.nameValuePair "browser/${name}.toml" {
          inherit source;
        }
      ) cfg.configFiles
      // lib.optionalAttrs (cfg.managedPolicyFile != null) {
        "browser/brave-managed-policy.json".source = cfg.managedPolicyFile;
      };
  };
}
