{
  lib,
  schema ? null,
}:

let
  productProviders = map (path: import path { inherit lib schema; }) [
    ../../addons/brave/nix
    ../../addons/helium/nix
  ];
  browserModules = map (provider: {
    options.${provider.name} = provider.settings;
  }) productProviders;
  engineProviders = [
    (import ../../addons/chromium/nix { inherit schema browserModules; })
  ];
  providers = engineProviders ++ productProviders;
in
{
  settingsModules = map (provider: {
    options = provider.settingsOptions or { };
  }) providers;
  hasPolicyIntent = settings: lib.any (provider: provider.hasPolicyIntent settings) providers;
  nixosModules = lib.concatMap (provider: provider.nixosModules or [ ]) providers;
  homeManagerModules = lib.concatMap (provider: provider.homeManagerModules or [ ]) providers;
  systemModules = lib.concatMap (provider: provider.systemModules or [ ]) providers;
}
