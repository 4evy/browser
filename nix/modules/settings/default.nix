{
  lib,
  tomlFormat,
}:

let
  schema = import ./lib.nix { inherit lib tomlFormat; };
  addons = import ../../providers { inherit lib schema; };
in
schema.types.submoduleWith {
  shorthandOnlyDefinesConfig = true;
  modules = addons.settingsModules;
}
