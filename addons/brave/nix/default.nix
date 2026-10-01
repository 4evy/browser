{ lib, schema }:

{
  name = "brave";
  settings = schema.optionalSubmodule (import ./settings.nix { inherit lib schema; }) ''
    Preferences implemented only by Brave.
  '';
  inherit (import ./policy.nix { inherit lib; }) hasPolicyIntent;
  nixosModules = [ ./nixos.nix ];
  homeManagerModules = [ ./home-manager.nix ];
  systemModules = [ ./system.nix ];
}
