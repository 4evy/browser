{ config, lib, ... }:

let
  cfg = config.programs.tweezle;
  policy = import ./policy.nix { inherit lib; };
  policyFile = cfg.managedPolicyFiles.chromium or null;
in
{
  config = lib.modules.mkIf (cfg.enable && policyFile != null && policy.isConfigured cfg) {
    environment.etc."brave/policies/managed/tweezle.json".source = policyFile;
  };
}
