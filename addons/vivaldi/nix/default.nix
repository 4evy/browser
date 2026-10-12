{ lib, schema }:

{
  name = "vivaldi";
  settings =
    schema.optionalSubmoduleWith [ (schema.optionsModule ./settings.nix { inherit lib schema; }) ]
      ''
        Preferences implemented only by Vivaldi.
      '';
  hasPolicyIntent = _: false;
}
