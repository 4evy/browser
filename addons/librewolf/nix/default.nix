{ lib, schema }:

{
  name = "librewolf";
  settings = schema.optionalSubmoduleWith [
    (schema.optionsModule ./settings.nix { inherit lib schema; })
  ] "Settings specific to librewolf.";
  hasPolicyIntent = _: false;
}
