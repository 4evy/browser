{ lib, schema }:

let
  metadata = lib.trivial.importJSON ../metadata.json;
  catalog = import ../../firefox/nix/catalog.nix { inherit lib schema; };
  inherit (schema)
    types
    optionalSubmodule
    optionalString
    optionalBool
    ;
in
{
  preferences = catalog.preferences metadata;
  shortcuts = schema.optional (types.attrsOf (
    schema.freeformSubmodule {
      key = optionalString "Character key; clears the existing keycode when nonempty.";
      keycode = optionalString "Gecko virtual key code; clears the existing character key when nonempty.";
      disabled = optionalBool "Disable this existing shortcut.";
      modifiers = optionalSubmodule (lib.attrsets.genAttrs [ "control" "alt" "shift" "meta" "accel" ] (
        _: optionalBool "Native shortcut modifier."
      )) "Modifier changes; omitted modifiers survive.";
    }
  )) "Patches keyed by existing shortcut ID; requires initialized version 21 shortcuts.";
  mods = schema.optional (types.attrsOf (
    schema.freeformSubmodule {
      enabled = optionalBool "Enable this Mod; defaults to the installed value or enabled for a new Mod.";
      revision = optionalString "Full theme-store commit SHA to pin the manifest and store-hosted assets.";
      style_sha256 = optionalString "Optional hexadecimal SHA-256 of the stylesheet.";
      preferences = schema.optional (types.attrsOf (
        types.oneOf [
          types.bool
          types.str
        ]
      )) "Mod preference values; checked against its downloaded preference definitions.";
    }
  )) "Official Zen Mods keyed by their UUID; installed assets and other Mods are preserved.";
  workspaces =
    schema.optional
      (types.attrsOf (
        schema.freeformSubmodule {
          name = optionalString "Workspace name; required for new workspaces.";
          icon = optionalString "Workspace icon.";
          containerTabId = schema.optionalUnsigned "Container ID, zero for the default container.";
          theme = optionalSubmodule {
            type = schema.optional (types.enum [ "gradient" ]) "Native workspace theme kind.";
            gradientColors = schema.optional (types.listOf (
              schema.freeformSubmodule { }
            )) "Native gradient color dot records.";
            opacity = schema.optional (types.oneOf [
              types.int
              types.float
            ]) "Theme opacity from zero to one.";
            texture = schema.optional (types.oneOf [
              types.int
              types.float
            ]) "Theme grain from zero to one.";
            rotation = schema.optional (types.oneOf [
              types.int
              types.float
            ]) "Gradient rotation.";
          } "Workspace theme patch; omitted fields survive.";
        }
      ))
      "Workspace patches keyed by UUID in initialized zen-sessions.jsonlz4; preserves tabs, folders, and split views.";
  workspace_order = schema.optionalStringList "Place these workspace UUIDs first in the supplied order, preserving remaining spaces.";
}
