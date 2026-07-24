{
  config,
  lib,
  ...
}:

let
  cfg = config.programs.browser;
in
{
  _file = ./home-manager.nix;
  _class = "homeManager";

  imports = [ ./options.nix ];

  config = lib.mkIf cfg.enable {
    home.packages = lib.optional (cfg.package != null) cfg.package;

    xdg.configFile = lib.mapAttrs' (
      name: source:
      lib.nameValuePair "browser/${name}.toml" {
        inherit source;
      }
    ) cfg.configFiles;
  };
}
