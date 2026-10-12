{ config, lib, ... }:

let
  cfg = config.programs.tweezle;
  policyFile = cfg.managedPolicyFiles.edge or null;
in
{
  config = lib.modules.mkIf (cfg.enable && policyFile != null) {
    environment.etc."opt/edge/policies/managed/tweezle.json".source = policyFile;
  };
}
