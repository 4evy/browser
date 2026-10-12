{ lib, schema }:

let
  metadata = lib.trivial.importJSON ../metadata.json;
  catalog = import ../../chromium/nix/policies.nix { inherit lib schema metadata; };
in
{
  name = "chrome";
  settings =
    schema.optionalSubmoduleWith [ (schema.optionsModule ./settings.nix { inherit lib schema; }) ]
      ''
        Enforced Google Chrome policies. Rendered policies must be installed in
        the browser's system policy store to take effect.
      '';
  hasPolicyIntent = settings: catalog.hasPolicyIntent (settings.chromium.chrome or null);
  nixosModules = [ ./nixos.nix ];
}
