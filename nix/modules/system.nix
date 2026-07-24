{
  config,
  lib,
  ...
}:

let
  cfg = config.programs.browser;
in
{
  _file = ./system.nix;
  _class = null;

  imports = [ ./options.nix ];

  config = lib.mkIf cfg.enable {
    environment.systemPackages = lib.optional (cfg.package != null) cfg.package;

    environment.etc = lib.mapAttrs' (
      name: source:
      lib.nameValuePair "browser/${name}.toml" {
        inherit source;
      }
    ) cfg.configFiles;
  };
}
