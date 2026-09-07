module.exports = {
  allowedCommands: [
    "^nix --extra-experimental-features 'nix-command flakes' run --inputs-from \\. nixpkgs#nix-update -- --flake --version=skip --build browser$",
  ],
  platform: "github",
  onboarding: false,
  requireConfig: "required",
  gitAuthor:
    "github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>",
};
