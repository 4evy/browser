{ lib, schema }:

{
  name = "helium";
  settings =
    schema.optionalSubmoduleWith [ (schema.optionsModule ./settings.nix { inherit lib schema; }) ]
      ''
        Preferences implemented only by Helium.
      '';
  hasPolicyIntent = _: false;
  nixosModules = [ ];
}
