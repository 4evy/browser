{ lib, schema }:

let
  metadata = lib.trivial.importJSON ../metadata.json;
  catalog = import ../../firefox/nix/catalog.nix { inherit lib schema; };
  feature =
    name:
    schema.optional (catalog.preferenceType metadata.preferences.${name}) ''
      Partial Floorp 12 native JSON configuration; objects merge with persisted
      preferences and explicit arrays replace. Omitted values retain defaults.
    '';
in
{
  preferences = catalog.preferences metadata;
  design = feature "floorp.design.configs";
  sidebar = feature "floorp.panelSidebar.config";
  panels = schema.optional (catalog.fromSchema
    metadata.preferences."floorp.panelSidebar.data".schema.properties.data
  ) "Complete panel list; replaces the existing list when configured.";
  keyboard = feature "floorp.keyboardshortcut.config";
  gestures = feature "floorp.mousegesture.config";
  workspaces = feature "floorp.workspaces.v4.config";
  workspace_data = feature "floorp.workspaces.v4.store";
  pwa = feature "floorp.browser.ssb.config";
  split_view = feature "floorp.splitView.config";
  pane_sizes = feature "floorp.splitView.paneSizes";
  context_menu = feature "floorp.contextMenu.config";
  newtab = feature "floorp.newtab.configs";
  memory = feature "floorp.memory.idleReclaim";
  command_palette = schema.optionalSubmodule (lib.attrsets.mapAttrs'
    (
      name: definition:
      lib.attrsets.nameValuePair (lib.strings.removePrefix "floorp.commandPalette." name) (
        schema.optional (catalog.preferenceType definition) "Command palette setting."
      )
    )
    (
      lib.attrsets.filterAttrs (
        name: _: lib.strings.hasPrefix "floorp.commandPalette." name
      ) metadata.preferences
    )
  ) "Command palette controls.";
}
