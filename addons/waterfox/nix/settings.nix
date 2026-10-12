{ lib, schema }:

let
  metadata = lib.trivial.importJSON ../metadata.json;
  catalog = import ../../firefox/nix/catalog.nix { inherit lib schema; };
  appearance = lib.trivial.importJSON ../appearance.json;
in
{
  preferences = catalog.preferences metadata;
  appearance = schema.optionalSubmodule (lib.attrsets.mapAttrs (
    _: choices:
    schema.optional (schema.types.enum (builtins.attrNames choices)) "Choose one appearance variant and clear conflicting toggles."
  ) appearance) "Waterfox appearance choice groups.";
}
