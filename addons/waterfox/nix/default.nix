{ lib, schema }:

{
  name = "waterfox";
  settings = schema.optionalSubmoduleWith [
    (schema.optionsModule ./settings.nix { inherit lib schema; })
  ] "Settings specific to waterfox.";
  hasPolicyIntent = _: false;
}
