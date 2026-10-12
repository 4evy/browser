{
  formatterPackage,
  pkgs,
  src,
}:

{
  formatting = pkgs.runCommandLocal "tweezle-formatting" {
    inherit src;
    nativeBuildInputs = [
      pkgs.biome
      pkgs.deadnix
      pkgs.findutils
      pkgs.go_1_27
      pkgs.statix
      formatterPackage
    ];
  } (builtins.readFile ./source/source.sh);

  extensionSettingsSchema = pkgs.runCommandLocal "tweezle-extension-settings-schema" {
    inherit src;
    nativeBuildInputs = [ pkgs.check-jsonschema ];
  } (builtins.readFile ./source/schema.sh);
}
