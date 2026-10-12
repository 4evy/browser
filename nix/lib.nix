{ lib }:

let
  removeNulls =
    value:
    if lib.attrsets.isDerivation value then
      value
    else if builtins.isAttrs value then
      lib.attrsets.mapAttrs (_: removeNulls) (lib.attrsets.filterAttrs (_: item: item != null) value)
    else if builtins.isList value then
      map removeNulls (lib.lists.filter (item: item != null) value)
    else
      value;

  chromiumSections = [
    "extensions"
    "extension_settings"
    "ublock"
  ];

  toTOML =
    {
      chromium ? null,
      firefox ? null,
    }:
    let
      cleaned = removeNulls { inherit chromium firefox; };
    in
    removeAttrs cleaned [ "chromium" ]
    // lib.attrsets.optionalAttrs (chromium != null) (
      {
        browser = removeAttrs cleaned.chromium chromiumSections;
      }
      // lib.attrsets.filterAttrs (name: _: lib.lists.elem name chromiumSections) cleaned.chromium
    );
in
{
  # Keep the Nix browser-family namespaces independent of the TOML layout
  inherit toTOML;

  /**
    Generate a TOML configuration file in the Nix store

    Chromium settings are grouped under `chromium` in Nix and serialized as the
    TOML `browser`, `extensions`, `extension_settings`, and `ublock` tables.
    Optional module settings use `null` to mean "not configured"; those values
    are removed recursively before serialization

    # Type

    ```
    generateConfig :: {
      pkgs :: AttrSet,
      settings :: AttrSet,
      name ? String
    } -> Path
    ```
  */
  generateConfig =
    {
      pkgs,
      settings,
      name ? "tweezle.toml",
    }:
    (pkgs.formats.toml { }).generate name (toTOML settings);
}
