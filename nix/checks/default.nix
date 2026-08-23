{
  browserPackage,
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
    inherit browserPackage pkgs;
  };

  moduleChecks = import ./modules.nix {
    inherit
      browserPackage
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
  version = browserPackage.passthru.tests.version;
}
