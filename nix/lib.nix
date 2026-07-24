{ lib }:

let
  removeNulls =
    value:
    if lib.isDerivation value then
      value
    else if builtins.isAttrs value then
      lib.mapAttrs (_: removeNulls) (lib.filterAttrs (_: item: item != null) value)
    else if builtins.isList value then
      map removeNulls (lib.filter (item: item != null) value)
    else
      value;
in
{
  /**
    Generate a TOML configuration file in the Nix store.

    Optional module settings use `null` to mean “not configured”; those values
    are removed recursively before serialization.

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
      name ? "browser.toml",
    }:
    (pkgs.formats.toml { }).generate name (removeNulls settings);
}
