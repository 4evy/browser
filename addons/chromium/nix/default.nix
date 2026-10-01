{ schema, browserModules }:

let
  chromiumSchema = schema // {
    extensionId = schema.types.strMatching "[a-p]{32}";
  };
in
{
  name = "chromium";
  settingsOptions = {
    browser = schema.required (schema.freeformSubmoduleWith (
      [ { options = import ./settings.nix { schema = chromiumSchema; }; } ] ++ browserModules
    )) "Chromium identity, platform metadata, paths, and preference defaults.";
  }
  // (import ./extensions.nix { schema = chromiumSchema; });
  hasPolicyIntent = _: false;
}
