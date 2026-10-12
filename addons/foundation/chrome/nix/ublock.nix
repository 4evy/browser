{ schema }:

let
  inherit (schema)
    optional
    optionalBool
    optionalString
    optionalStringList
    optionalSubmodule
    pathString
    types
    ;

  installation = {
    install = optionalBool "Install the extension; defaults to true except for Helium's bundled Origin, which is configured directly.";
    id = optional schema.extensionId "Existing extension ID when install is false.";
    settings_file = optional pathString "Settings JSON file, including Origin's Back up to file export; inline TOML overrides its fields.";
    managed = optional (types.attrsOf types.anything) "Upstream managed settings included in policy output.";
  };

  scalarSettings = optional (
    types.attrsOf (
      types.oneOf [
        types.bool
        types.int
        types.float
        types.str
      ]
    )
  );
in
{
  ublock = optionalSubmodule {
    origin =
      optionalSubmodule
        (
          installation
          // {
            extension_dir = optional pathString "Existing unpacked Origin directory when install is false.";
            builtin = optionalBool "Configure a browser component that loads in temporary profiles.";
            user_settings = optional (types.attrsOf types.anything) "Origin dashboard user settings using upstream names.";
            hidden_settings = scalarSettings "Origin advanced settings using upstream names.";
            filter_lists = optionalStringList "Replace selected filter list IDs and imported list URLs.";
            trusted_sites = optionalStringList "Replace trusted site directives; an empty list clears them.";
            user_filters = optionalString "Replace My filters; an empty string clears them.";
            rules = optionalString "Replace permanent My rules; an empty string clears them.";
          }
        )
        "Configure uBlock Origin from JSON; Helium uses its bundled component automatically. Requires Manifest V2 support.";

    lite = optionalSubmodule (
      installation
      // {
        rulesets = optionalStringList "Replace enabled Lite ruleset IDs.";
        auto_reload = optionalBool "Reload pages after changing their filtering mode.";
        show_blocked_count = optionalBool "Show the blocked request count on the toolbar icon.";
        strict_block_mode = optionalBool "Enable strict blocking.";
        popup_block_mode = optionalBool "Enable popup blocking.";
        developer_mode = optionalBool "Enable Lite developer mode.";
        filtering_modes = optionalSubmodule {
          none = optionalStringList "Hostnames with no filtering.";
          basic = optionalStringList "Hostnames with basic filtering.";
          optimal = optionalStringList "Hostnames with optimal filtering.";
          complete = optionalStringList "Hostnames with complete filtering.";
        } "Replace site modes; put all-urls in exactly one mode for the default.";
      }
    ) "Native uBlock Origin Lite installation and settings for Manifest V3.";
  } "First-class uBlock settings shared by all Chromium-family browsers.";
}
