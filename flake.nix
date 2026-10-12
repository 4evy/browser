{
  description = "Declarative configuration for Chromium and Firefox-family browsers";

  inputs = {
    nixpkgs.url = "https://channels.nixos.org/nixos-unstable/nixexprs.tar.zst";

    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };

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
    inputs.flake-parts.lib.mkFlake { inherit inputs; } (
      { lib, ... }: {
        imports = [ ./nix/checks.nix ];

        systems = [
          "aarch64-darwin"
          "aarch64-linux"
          "x86_64-linux"
        ];

        perSystem =
          { config, pkgs, ... }:
          {
            packages = {
              tweezle = pkgs.callPackage ./nix/package.nix { };
              default = config.packages.tweezle;
            };

            apps.tweezle = {
              type = "app";
              program = lib.meta.getExe config.packages.tweezle;
              meta.description = "Configure a Chromium or Firefox-family browser";
            };
            apps.default = config.apps.tweezle;

            devShells.default = pkgs.mkShell {
              inputsFrom = [ config.packages.tweezle ];
              packages = builtins.attrValues {
                inherit (pkgs)
                  bun
                  deadnix
                  go_1_27
                  gopls
                  nixfmt
                  statix
                  ;
                inherit (config) formatter;
                inherit (config.packages.tweezle.passthru) golangciLint;
              };
            };

            formatter = pkgs.nixfmt-tree.override {
              settings.excludes = [ ".sources/**" ];
            };
          };

        flake = {
          overlays.default = final: _prev: {
            tweezle = final.callPackage ./nix/package.nix { };
          };

          homeModules = {
            tweezle = ./nix/modules/home.nix;
            default = inputs.self.homeModules.tweezle;
          };
          nixosModules = {
            tweezle = ./nix/modules/nixos.nix;
            default = inputs.self.nixosModules.tweezle;
          };
          darwinModules = {
            tweezle = ./nix/modules/darwin.nix;
            default = inputs.self.darwinModules.tweezle;
          };

          lib = import ./nix/lib.nix { inherit lib; };
        };
      }
    );
}
