{
  lib,
  buildGoModule,
  testers,
}:

buildGoModule (finalAttrs: {
  pname = "browser";
  version = "0-unstable-2026-07-24";

  src = lib.fileset.toSource {
    root = ./.;
    fileset = lib.fileset.unions [
      (lib.fileset.fileFilter (file: file.hasExt "go") ./.)
      ./go.mod
      ./go.sum
      ./testdata
    ];
  };

  vendorHash = "sha256-8ebGxIvR9kKp/yrDrGJYsFnNAH3lPaFBANy4YiXj0QY=";

  subPackages = [ "cmd/browser" ];

  checkPhase = ''
    runHook preCheck
    go test ./...
    runHook postCheck
  '';

  ldflags = [
    "-s"
    "-w"
    "-X main.version=${finalAttrs.version}"
  ];

  passthru.tests.version = testers.testVersion {
    package = finalAttrs.finalPackage;
  };

  meta = {
    description = "Generic Chromium-family browser configurator";
    homepage = "https://github.com/4evy/browser";
    license = lib.licenses.mit;
    mainProgram = "browser";
    platforms = lib.platforms.unix;
  };
})
