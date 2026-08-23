{
  formatterPackage,
  pkgs,
  src,
}:

let
  nodeModules = pkgs.importNpmLock.buildNodeModules {
    npmRoot = src;
    inherit (pkgs) nodejs;
  };
in
{
  formatting =
    pkgs.runCommandLocal "browser-formatting"
      {
        nativeBuildInputs = [
          pkgs.deadnix
          pkgs.findutils
          pkgs.go_1_27
          pkgs.statix
          formatterPackage
        ];
      }
      ''
        cp -R ${src} source
        chmod -R u+w source

        unformatted="$(find source -name '*.go' -type f -print0 | xargs -0 gofmt -l)"
        if [ -n "$unformatted" ]; then
          echo "Go files need formatting:" >&2
          echo "$unformatted" >&2
          exit 1
        fi

        (
          cd source
          ${nodeModules}/node_modules/.bin/prettier --check .
          treefmt --ci --walk filesystem --tree-root "$PWD"
          deadnix --fail .

          statixOutput="$(statix check . 2>&1)"
          if [ -n "$statixOutput" ]; then
            echo "$statixOutput" >&2
            exit 1
          fi
        )
        touch "$out"
      '';

  extensionSettingsSchema =
    pkgs.runCommandLocal "browser-extension-settings-schema"
      {
        nativeBuildInputs = [ pkgs.check-jsonschema ];
      }
      ''
        check-jsonschema \
          --schemafile ${src}/schema/extension-settings.schema.json \
          ${src}/testdata/extension-settings/comprehensive.json
        touch "$out"
      '';
}
