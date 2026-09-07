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
  pname = "browser";
  version = "0-unstable-2026-08-23";

  src = lib.fileset.toSource {
    root = ./..;
    fileset = lib.fileset.unions [
      (lib.fileset.fileFilter (file: file.hasExt "go") ./..)
      ../go.mod
      ../go.sum
      ../internal/browsercore/data
      ../testdata
    ];
  };

  vendorHash = "sha256-g8T2JC4Q1gJT1SXktIyJl13tuXOZd+mfjkxQysr5OCE=";

  subPackages = [ "cmd/browser" ];

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
    installShellCompletion --cmd browser \
      --bash <("$out/bin/browser" completion bash) \
      --zsh <("$out/bin/browser" completion zsh) \
      --fish <("$out/bin/browser" completion fish)
  '';

  postFixup = ''
    wrapProgram "$out/bin/browser" \
      --prefix PATH : ${lib.makeBinPath [ gitMinimal ]}
  '';

  passthru = {
    golangciLint = golangci-lint;

    tests.version = testers.testVersion {
      package = finalAttrs.finalPackage;
    };
  };

  meta = {
    description = "Generic Chromium-family browser configurator";
    homepage = "https://github.com/4evy/browser";
    license = lib.licenses.mit;
    mainProgram = "browser";
    maintainers = [ lib.maintainers._4evy ];
    platforms = lib.platforms.unix;
  };
})
