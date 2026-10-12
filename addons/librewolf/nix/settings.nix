{ lib, schema }:

let
  metadata = lib.trivial.importJSON ../metadata.json;
  catalog = import ../../firefox/nix/catalog.nix { inherit lib schema; };
  preferences = catalog.preferences metadata;
in
{
  inherit preferences;
  override_file = schema.optionalString ''
    LibreWolf's actual librewolf.overrides.cfg destination. Required for
    defaults, overrides, locked, or unlock; applies to every profile loaded
    by that installation. Existing content outside the marked block survives.
    Supports home and config_home templates. Choose the installation's path.
  '';
  defaults = preferences // {
    description = "AutoConfig defaultPref values; users may override them.";
  };
  overrides = preferences // {
    description = "AutoConfig pref values enforced on each startup.";
  };
  locked = preferences // {
    description = "AutoConfig lockPref values locked against user changes.";
  };
  unlock = schema.optionalStringList "Native preferences to unlock before applying overrides.";
}
