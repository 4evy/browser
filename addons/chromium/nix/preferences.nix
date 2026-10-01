{ schema }:

let
  inherit (schema)
    freeformSubmodule
    optional
    optionalStringList
    optionalSubmodule
    required
    types
    ;

  preferenceValue = freeformSubmodule {
    path = required types.nonEmptyStr "Dotted path of the value to update.";
    value = required types.toml "TOML value to write at `path`.";
  };

  accelerator = freeformSubmodule {
    path = required types.nonEmptyStr ''
      Dotted path containing Chromium custom accelerators.
    '';
    command_id = required types.nonEmptyStr "Chromium command identifier.";
    accelerator = required types.nonEmptyStr "Accelerator string to add.";
  };
in
{
  values = optional (types.listOf preferenceValue) "Values written to Chromium Preferences.";
  local_state_values = optional (types.listOf preferenceValue) ''
    Values written to Chromium Local State.
  '';
  variation_values = optional (types.listOf preferenceValue) ''
    Values written to Chromium Variations.
  '';
  accelerators = optional (types.listOf accelerator) ''
    Custom accelerator entries added to Preferences.
  '';

  cookies = optionalSubmodule {
    default = optional (types.enum [
      "allow"
      "block"
      "session_only"
    ]) "Default cookie behavior.";
    third_party = optional (types.enum [
      "off"
      "block"
      "incognito_only"
    ]) "Chromium cookie-controls mode for third-party cookies.";
    allow = optionalStringList ''
      Cookie patterns to allow; an empty list removes existing allow exceptions.
    '';
    block = optionalStringList ''
      Cookie patterns to block; an empty list removes existing block exceptions.
    '';
    session_only = optionalStringList "Cookie patterns retained only for the current session.";
  } "Default, third-party, and per-site cookie policy.";
}
