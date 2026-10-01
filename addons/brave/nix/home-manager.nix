{ config, lib, ... }:

let
  cfg = config.programs.browser;
  policy = import ./policy.nix { inherit lib; };
in
{
  # Preserve the established Brave policy artifact path for existing consumers
  config = lib.mkIf (cfg.enable && cfg.managedPolicyFile != null && policy.isConfigured cfg) {
    xdg.configFile."browser/brave-managed-policy.json".source = cfg.managedPolicyFile;
  };
}
