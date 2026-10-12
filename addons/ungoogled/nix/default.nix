{ schema, ... }:

{
  name = "ungoogled";
  settings =
    schema.optionalSubmoduleWith [ (schema.optionsModule ./settings.nix { inherit schema; }) ]
      ''
        Custom launch switches implemented by ungoogled Chromium.
        Boolean switches are emitted only when true; false and null emit nothing.
        These options do not change saved chrome://flags selections.
      '';
  hasPolicyIntent = _: false;
}
