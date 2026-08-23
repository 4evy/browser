{
  lib,
  tomlFormat,
}:

let
  schema = import ./lib.nix { inherit lib tomlFormat; };

  browserOptions = import ./browser.nix { inherit schema; };
  extensionOptions = import ./extensions.nix { inherit schema; };
in
schema.freeformSubmodule (
  {
    browser = schema.required (schema.freeformSubmodule browserOptions) ''
      Browser identity, platform metadata, paths, and preference defaults.
    '';
  }
  // extensionOptions
)
