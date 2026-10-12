{
  tweezlePackage,
  darwinModule,
  formatterPackage,
  homeManager,
  homeManagerModule,
  lib,
  nixDarwin,
  nixosModule,
  pkgs,
  src,
}:

let
  fixtures = import ./fixtures.nix {
    inherit tweezlePackage pkgs;
  };

  moduleChecks = import ./modules.nix {
    inherit
      tweezlePackage
      darwinModule
      fixtures
      homeManager
      homeManagerModule
      lib
      nixDarwin
      nixosModule
      pkgs
      ;
  };

  sourceChecks = import ./source.nix {
    inherit formatterPackage pkgs src;
  };
in
sourceChecks
// moduleChecks
// {
  version = tweezlePackage.passthru.tests.version;
}
