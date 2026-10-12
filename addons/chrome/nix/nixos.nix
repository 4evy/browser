{ config, lib, ... }:

let
  cfg = config.programs.tweezle;
  policyFile = cfg.managedPolicyFiles.chrome or null;
in
{
  config = lib.modules.mkIf (cfg.enable && policyFile != null) {
    environment.etc."opt/chrome/policies/managed/tweezle.json".source = policyFile;
  };
}
