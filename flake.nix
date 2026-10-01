{
  description = "Declarative configuration for Chromium-family browsers";

  inputs = {
    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };

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
    inputs@{ flake-parts, self, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } (
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
              browser = pkgs.callPackage ./nix/package.nix { };
              default = config.packages.browser;
            };

            apps.browser = {
              type = "app";
              program = lib.getExe config.packages.browser;
              meta.description = "Configure a Chromium-family browser";
            };
            apps.default = config.apps.browser;

            devShells.default = pkgs.mkShell {
              inputsFrom = [ config.packages.browser ];
              packages = builtins.attrValues {
                inherit (pkgs)
                  deadnix
                  go_1_27
                  gopls
                  nixfmt
                  nodejs_24
                  statix
                  ;
                inherit (config) formatter;
                inherit (config.packages.browser.passthru) golangciLint;
              };
            };

            formatter = pkgs.nixfmt-tree.override {
              settings.excludes = [ ".sources/**" ];
            };
          };

        flake = {
          overlays.default = final: _prev: {
            browser = final.callPackage ./nix/package.nix { };
          };

          homeModules = {
            browser = ./nix/modules/home-manager.nix;
            default = self.homeModules.browser;
          };
          nixosModules = {
            browser = ./nix/modules/nixos.nix;
            default = self.nixosModules.browser;
          };
          darwinModules = {
            browser = ./nix/modules/nix-darwin.nix;
            default = self.darwinModules.browser;
          };

          # Kept for compatibility with the older community output name
          homeManagerModules = self.homeModules;
          lib = import ./nix/lib.nix { inherit lib; };
        };
      }
    );
}
