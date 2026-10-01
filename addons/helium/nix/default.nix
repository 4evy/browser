{ lib, schema }:

{
  name = "helium";
  settings = schema.optionalSubmodule (import ./settings.nix { inherit lib schema; }) ''
    Preferences implemented only by Helium.
  '';
  hasPolicyIntent = _: false;
  nixosModules = [ ];
}
