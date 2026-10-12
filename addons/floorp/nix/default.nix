{ lib, schema }:

{
  name = "floorp";
  settings = schema.optionalSubmoduleWith [
    (schema.optionsModule ./settings.nix { inherit lib schema; })
  ] "Settings specific to floorp.";
  hasPolicyIntent = _: false;
}
