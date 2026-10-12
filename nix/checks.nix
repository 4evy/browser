{ inputs, self, ... }:
{
  perSystem =
    { config, pkgs, ... }:
    {
      checks = pkgs.callPackages ./checks {
        tweezlePackage = config.packages.tweezle;
        darwinModule = self.darwinModules.tweezle;
        formatterPackage = config.formatter;
        homeManager = inputs.home-manager;
        homeManagerModule = self.homeModules.tweezle;
        inherit (inputs.nixpkgs) lib;
        nixDarwin = inputs.nix-darwin;
        nixosModule = self.nixosModules.tweezle;
        src = self;
      };
    };
}
