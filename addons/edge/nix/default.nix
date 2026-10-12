{ lib, schema }:

let
  metadata = lib.trivial.importJSON ../metadata.json;
  catalog = import ../../chromium/nix/policies.nix { inherit lib schema metadata; };
in
{
  name = "edge";
  settings =
    schema.optionalSubmoduleWith [ (schema.optionsModule ./settings.nix { inherit lib schema; }) ]
      ''
        Enforced Microsoft Edge policies. Rendered policies must be installed in
        the browser's system policy store to take effect.
      '';
  hasPolicyIntent = settings: catalog.hasPolicyIntent (settings.chromium.edge or null);
  nixosModules = [ ./nixos.nix ];
}
