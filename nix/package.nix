{
  lib,
  buildGo127Module,
  gitMinimal,
  golangci-lint,
  installShellFiles,
  makeWrapper,
  testers,
}:

buildGo127Module (finalAttrs: {
  pname = "tweezle";
  version = "0-unstable-2026-08-23";

  src = lib.fileset.toSource {
    root = ./..;
    fileset = lib.fileset.unions [
      (lib.fileset.fileFilter (file: file.hasExt "go") ./..)
      ../go.mod
      ../go.sum
      (lib.fileset.fileFilter (file: file.hasExt "json" || file.hasExt "js") ../addons)
      (lib.fileset.fileFilter (file: file.hasExt "json" || file.hasExt "toml") ../examples)
      ../addons/brave/testdata
      ../addons/chromium/testdata
      ../addons/foundation/chrome/extensions/testdata
      ../addons/foundation/chrome/storage/testdata
      ../addons/helium/testdata
      ../cli/testdata
      ../cli/examples
      ../desktop/testdata
      ../internal/file/testdata
      ../launcher/testdata
      ../testdata
    ];
  };

  vendorHash = "sha256-4bFRXT2EqUt6RErWjqhrqd2jy+LEAY48mUA45w251ks=";

  subPackages = [ "cmd/tweezle" ];

  nativeBuildInputs = [
    installShellFiles
    makeWrapper
  ];

  nativeCheckInputs = [
    golangci-lint
    gitMinimal
  ];

  checkPhase = ''
    runHook preCheck
    export GOLANGCI_LINT_CACHE="$TMPDIR/golangci-lint-cache"
    go fix -diff ./...
    go vet ./...
    go test ./...
    golangci-lint run ./...
    runHook postCheck
  '';

  ldflags = [
    "-s"
    "-X main.version=${finalAttrs.version}"
  ];

  postInstall = ''
    installShellCompletion --cmd tweezle \
      --bash <("$out/bin/tweezle" completion bash) \
      --zsh <("$out/bin/tweezle" completion zsh) \
      --fish <("$out/bin/tweezle" completion fish)
  '';

  postFixup = ''
    wrapProgram "$out/bin/tweezle" \
      --prefix PATH : ${lib.strings.makeBinPath [ gitMinimal ]}
  '';

  passthru = {
    golangciLint = golangci-lint;

    tests.version = testers.testVersion {
      package = finalAttrs.finalPackage;
    };
  };

  meta = {
    description = "Generic Chromium and Firefox-family browser configurator";
    homepage = "https://github.com/4evy/tweezle";
    license = lib.licenses.mit;
    mainProgram = "tweezle";
    maintainers = [ lib.maintainers._4evy ];
    platforms = lib.systems.doubles.unix;
  };
})
