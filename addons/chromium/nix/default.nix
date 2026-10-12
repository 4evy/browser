{
  lib,
  schema,
  browserModules,
}:

let
  chromiumSchema = schema // {
    extensionId = schema.types.strMatching "[a-p]{32}";
  };
  extensionModule = schema.optionsModule ../../foundation/chrome/nix/extensions.nix {
    schema = chromiumSchema;
  };
  ublockModule = schema.optionsModule ../../foundation/chrome/nix/ublock.nix {
    schema = chromiumSchema;
  };
in
{
  name = "chromium";
  settingsModules = [
    {
      options.chromium = schema.optionalSubmoduleWith (
        [ (schema.optionsModule ../../foundation/chrome/nix/settings.nix { schema = chromiumSchema; }) ]
        ++ browserModules
        ++ [
          extensionModule
          ublockModule
        ]
      ) "Chromium-family identity, preferences, extension sources, and extension settings.";
    }
  ];
  hasPolicyIntent =
    settings:
    let
      ublock = settings.chromium.ublock or null;
      origin = if ublock == null then null else ublock.origin or null;
      lite = if ublock == null then null else ublock.lite or null;
      hasManaged =
        variant: variant != null && (variant.managed or null) != null && variant.managed != { };
    in
    hasManaged origin || hasManaged lite || (lite != null && (lite.install or null) != false);

  # Profile input paths must be available before validating in a build sandbox
  configurationCanBeChecked =
    settings:
    let
      extensionSettings = lib.trivial.defaultTo { } (settings.chromium.extension_settings or null);
      files = lib.trivial.defaultTo [ ] (extensionSettings.files or null);
      runtime = lib.trivial.defaultTo [ ] (extensionSettings.runtime or null);
      ublock = lib.trivial.defaultTo { } (settings.chromium.ublock or null);
      settingsFiles =
        lib.lists.concatMap
          (
            variant:
            lib.lists.optional (
              variant != null && (variant.settings_file or null) != null
            ) variant.settings_file
          )
          [
            (ublock.origin or null)
            (ublock.lite or null)
          ];
      runtimeFiles = lib.lists.concatMap (
        entry: [ entry.script ] ++ lib.attrsets.attrValues (lib.trivial.defaultTo { } (entry.files or null))
      ) runtime;
    in
    lib.lists.all lib.types.pathInStore.check (files ++ runtimeFiles ++ settingsFiles);
}
