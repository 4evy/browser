{ schema }:

let
  inherit (schema)
    optional
    optionalNonEmptyString
    optionalString
    optionalStringList
    optionalSubmodule
    optionalSubmoduleWith
    required
    types
    ;
in
{
  variant = optionalNonEmptyString ''
    Identity preset: firefox, beta, esr, developer, nightly, librewolf,
    waterfox, floorp, or zen. Unlisted Gecko variants work with explicit
    application metadata. Defaults to firefox.
  '';
  name = optionalNonEmptyString "Human-readable browser name; defaults to the variant name.";
  executable_name = optionalNonEmptyString "Generated launcher name; defaults to VARIANT-configured.";
  flags_file = optionalString "Optional flags file read by the generated launcher.";
  flags = optionalStringList "Cross-platform browser arguments; profile_dir selects the profile.";
  linux = optionalSubmodule {
    flatpak =
      optionalSubmoduleWith
        [
          (schema.optionsModule ../../../../nix/modules/settings/flatpak.nix { inherit schema; })
        ]
        ''
          Launch an installed Flatpak instead of a native executable.
          Profile paths refer to host files under ~/.var/app/APP_ID.
        '';
    app_dir = optionalString "Application directory; omit to find the launcher on PATH.";
    launcher_name = optionalNonEmptyString "Executable inside app_dir, or on PATH when app_dir is omitted.";
  } "Linux application metadata; preserve a fork's wrapper when one is provided.";
  macos = optionalSubmodule {
    app_dir = optionalString "macOS application bundle path.";
    launcher_path = optionalNonEmptyString "Launcher path relative to the application bundle.";
  } "macOS application metadata.";
  paths = optional (types.attrsOf (
    schema.freeformSubmodule {
      profile_dir = optionalString "Existing profile directory from about:profiles; also selected by the launcher.";
      policies_file = optionalString ''
        Actual policies.json destination loaded by the browser. This file is
        owned in full by the configuration and replaced on apply. Required to
        apply native policies or install extensions; policy render needs no
        path. Flatpak defaults to the package's systemconfig extension point.
      '';
    }
  )) "Paths for linux and macos; home, config_home, and data_home templates expand at runtime.";
  preferences = optional (types.attrsOf (
    types.oneOf [
      types.bool
      types.str
      (types.ints.between (-2147483648) 2147483647)
    ]
  )) "Preference names and scalar values written to a managed block in user.js.";
  policies = optional (schema.freeformSubmodule
    { }
  ) "Upstream enterprise policy values, without the outer policies envelope.";
  extensions = optional (types.listOf (
    schema.freeformSubmodule {
      id = required types.nonEmptyStr "Firefox add-on ID, including its braces or email-style suffix.";
      install_url = optionalNonEmptyString ''
        HTTPS or absolute file URL for an XPI; Firefox verifies and installs
        it on startup. Builds supporting URL-free installation can install
        by ID from Mozilla's add-on repository when both URLs are omitted.
      '';
      update_url = optionalNonEmptyString ''
        HTTPS or absolute file URL for an add-on update manifest, on Firefox
        builds that support this policy. Used for installation when install_url
        is omitted; also overrides the extension's update URL.
      '';
      installation_mode = optional (types.enum [
        "normal_installed"
        "force_installed"
      ]) "Installation mode; defaults to normal_installed.";
      updates_disabled = schema.optionalBool "Whether Firefox disables updates for this extension.";
    }
  )) "Extensions translated into the ExtensionSettings policy.";
  extension_settings = optional (types.attrsOf (schema.freeformSubmodule { })) ''
    Managed extension values keyed by Firefox add-on ID. Extensions must
    support storage.managed; these values do not modify local or sync storage.
  '';
}
