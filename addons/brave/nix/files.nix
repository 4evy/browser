{ config, lib, ... }:

let
  cfg = config.programs.tweezle;
  policy = import ./policy.nix { inherit lib; };
  policyFile = cfg.managedPolicyFiles.chromium or null;
in
{
  config = lib.modules.mkIf (cfg.enable && policyFile != null && policy.isConfigured cfg) {
    programs.tweezle.fileSources."tweezle/brave-managed-policy.json" = policyFile;
  };
}
