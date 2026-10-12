{ lib, schema }:

let
  metadata = lib.trivial.importJSON ../metadata.json;
  catalog = import ../../chromium/nix/policies.nix { inherit lib schema metadata; };
in
catalog.settings
// {
  managed_policies = schema.optional (schema.types.attrsOf schema.types.anything) ''
    Additional enforced policies by their official names. Values must agree
    with named options. Check the vendor reference for version, platform,
    and account restrictions.
  '';
}
