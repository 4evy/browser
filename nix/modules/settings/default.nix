{
  lib,
  tomlFormat,
}:

let
  schema = import ./lib.nix { inherit lib tomlFormat; };
  addons = import ../../providers { inherit lib schema; };
in
schema.freeformSubmoduleWith addons.settingsModules
