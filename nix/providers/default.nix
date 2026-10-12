{
  lib,
  schema ? null,
}:

let
  loadProvider =
    path: args:
    (import path ({ inherit lib schema; } // args))
    // {
      moduleLocation = "${toString path}/default.nix";
    };
  providerModule =
    provider: module: lib.modules.setDefaultModuleLocation provider.moduleLocation module;
  productProviders = map (path: loadProvider path { }) [
    ../../addons/brave/nix
    ../../addons/chrome/nix
    ../../addons/edge/nix
    ../../addons/helium/nix
    ../../addons/vivaldi/nix
    ../../addons/ungoogled/nix
  ];
  browserModules = map (
    provider:
    providerModule provider {
      options.${provider.name} = provider.settings;
    }
  ) productProviders;
  forkProviders = map (path: loadProvider path { }) [
    ../../addons/librewolf/nix
    ../../addons/waterfox/nix
    ../../addons/zen/nix
    ../../addons/floorp/nix
  ];
  forkModules = map (
    provider:
    providerModule provider {
      options.${provider.name} = provider.settings;
    }
  ) forkProviders;
  engineProviders = [
    (loadProvider ../../addons/chromium/nix { inherit browserModules; })
    (loadProvider ../../addons/firefox/nix { inherit forkModules; })
  ];
  providers = engineProviders ++ productProviders ++ forkProviders;
in
{
  settingsModules = lib.lists.concatMap (
    provider: map (providerModule provider) (provider.settingsModules or [ ])
  ) providers;
  configurationCanBeChecked =
    settings:
    lib.lists.all (provider: (provider.configurationCanBeChecked or (_: true)) settings) providers;
  hasPolicyIntent = settings: lib.lists.any (provider: provider.hasPolicyIntent settings) providers;
  nixosModules = lib.lists.concatMap (provider: provider.nixosModules or [ ]) providers;
  homeManagerModules = lib.lists.concatMap (provider: provider.homeManagerModules or [ ]) providers;
  systemModules = lib.lists.concatMap (provider: provider.systemModules or [ ]) providers;
}
