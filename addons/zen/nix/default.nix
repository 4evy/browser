{ lib, schema }:

{
  name = "zen";
  settings = schema.optionalSubmoduleWith [
    (schema.optionsModule ./settings.nix { inherit lib schema; })
  ] "Settings specific to zen.";
  hasPolicyIntent = _: false;
}
