{ config, lib, ... }:

let
  cfg = config.programs.browser;
  policy = import ./policy.nix { inherit lib; };
in
{
  config = lib.mkIf (cfg.enable && cfg.managedPolicyFile != null && policy.isConfigured cfg) {
    environment.etc."brave/policies/managed/browser.json".source = cfg.managedPolicyFile;
  };
}
