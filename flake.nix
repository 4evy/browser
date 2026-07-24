{
  description = "Declarative configuration for Chromium-family browsers";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    nix-darwin = {
      url = "github:nix-darwin/nix-darwin";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    inputs:
    let
      inherit (inputs.nixpkgs) lib;

      supportedSystems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];

      withDefault = browser: {
        inherit browser;
        default = browser;
      };

      homeModules = withDefault ./nix/modules/home-manager.nix;
      nixosModules = withDefault ./nix/modules/nixos.nix;
      darwinModules = withDefault ./nix/modules/nix-darwin.nix;

      perSystem =
        system:
        let
          pkgs = inputs.nixpkgs.legacyPackages.${system};
          browser = pkgs.callPackage ./package.nix { };
          formatter = pkgs.nixfmt-tree.override {
            settings.excludes = [ ".sources/**" ];
          };
          browserApp = {
            type = "app";
            program = lib.getExe browser;
            meta.description = "Configure a Chromium-family browser";
          };
        in
        {
          packages = withDefault browser;

          checks = import ./nix/checks.nix {
            inherit lib pkgs;
            browserPackage = browser;
            darwinModule = darwinModules.browser;
            homeManager = inputs.home-manager;
            homeManagerModule = homeModules.browser;
            nixDarwin = inputs.nix-darwin;
            nixosModule = nixosModules.browser;
            formatterPackage = formatter;
            src = inputs.self;
          };

          apps = withDefault browserApp;

          devShells.default = pkgs.mkShell {
            inputsFrom = [ browser ];
            packages = [
              pkgs.go
              pkgs.gopls
              formatter
            ];
          };

          inherit formatter;
        };

      perSystemOutputs = lib.genAttrs supportedSystems perSystem;
      outputFor = name: lib.mapAttrs (_: outputs: outputs.${name}) perSystemOutputs;
    in
    {
      packages = outputFor "packages";
      checks = outputFor "checks";
      apps = outputFor "apps";
      devShells = outputFor "devShells";
      formatter = outputFor "formatter";

      overlays.default = final: _prev: {
        browser = final.callPackage ./package.nix { };
      };

      inherit homeModules nixosModules darwinModules;
      homeManagerModules = homeModules;
      lib = import ./nix/lib.nix { inherit lib; };
    };
}
