#!/usr/bin/env bash
set -euo pipefail

# gomodUpdateImportPaths needs mod when binarySource is global
nix shell --inputs-from . nixpkgs#renovate nixpkgs#go_1_27 \
  nixpkgs#mod nixpkgs#nodejs_24 --command renovate
