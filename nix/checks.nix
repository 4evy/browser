{ inputs, self, ... }:
{
  perSystem =
    { config, pkgs, ... }:
    {
      checks = pkgs.callPackages ./checks {
        browserPackage = config.packages.browser;
        darwinModule = self.darwinModules.browser;
        formatterPackage = config.formatter;
        homeManager = inputs.home-manager;
        homeManagerModule = self.homeModules.browser;
        inherit (inputs.nixpkgs) lib;
        nixDarwin = inputs.nix-darwin;
        nixosModule = self.nixosModules.browser;
        src = self;
      };
    };
}
