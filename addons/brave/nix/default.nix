{ lib, schema }:

{
  name = "brave";
  settings =
    schema.optionalSubmoduleWith [ (schema.optionsModule ./settings.nix { inherit lib schema; }) ]
      ''
        Preferences implemented only by Brave.
      '';
  inherit (import ./policy.nix { inherit lib; }) hasPolicyIntent;
  nixosModules = [ ./nixos.nix ];
  homeManagerModules = [ ./files.nix ];
  systemModules = [ ./files.nix ];
}
