{
  lib,
  schema,
  forkModules ? [ ],
}:

{
  name = "firefox";
  settingsModules = [
    {
      options.firefox =
        schema.optionalSubmoduleWith
          (
            [ (schema.optionsModule ../../foundation/firefox/nix/settings.nix { inherit schema; }) ]
            ++ forkModules
          )
          ''
            Firefox-family identity, profile preferences, extensions, and policies.
          '';
    }
  ];
  hasPolicyIntent =
    settings:
    let
      firefox = settings.firefox or null;
    in
    firefox != null
    && lib.lists.any (value: value != null && value != { } && value != [ ]) [
      (firefox.policies or null)
      (firefox.extensions or null)
      (firefox.extension_settings or null)
    ];
}
