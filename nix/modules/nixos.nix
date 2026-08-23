{
  config,
  lib,
  ...
}:

{
  _class = "nixos";

  imports = [ ./system.nix ];

  config =
    lib.mkIf (config.programs.browser.enable && config.programs.browser.managedPolicyFile != null)
      {
        environment.etc."brave/policies/managed/browser.json".source =
          config.programs.browser.managedPolicyFile;
      };
}
