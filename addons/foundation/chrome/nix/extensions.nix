{ schema }:

let
  inherit (schema)
    freeformSubmodule
    optional
    optionalNonEmptyString
    optionalString
    optionalUnsigned
    pathString
    required
    types
    ;

  namedExtension = {
    id = required schema.extensionId "Chromium extension identifier." // {
      example = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
    };
    name = optionalNonEmptyString "Human-readable extension name.";
  };

  chromeStoreExtension = freeformSubmodule namedExtension;

  updateUrlExtension = freeformSubmodule (
    namedExtension
    // {
      update_url = required types.nonEmptyStr "Extension update manifest URL.";
    }
  );

  downloadedExtension = freeformSubmodule (
    namedExtension
    // {
      version = required (types.strMatching "[0-9]+(\\.[0-9]+)*") "Extension version." // {
        example = "1.2.3";
      };

      url = required types.nonEmptyStr "CRX download URL.";

      sha256 =
        required types.nonEmptyStr ''
          CRX SHA-256 checksum in SRI, raw hex, or `sha256:<hex>` form.
        ''
        // {
          example = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
        };
    }
  );

  zipExtension = freeformSubmodule (
    namedExtension
    // {
      update_policy = optional (types.enum [
        "latest"
        "pinned"
      ]) "Use the latest GitHub release or a version/URL/SHA-256 pinned artifact.";

      version = optional (types.strMatching "[0-9]+(\\.[0-9]+)*") "Pinned extension version.";
      url = optionalNonEmptyString "Pinned ZIP download URL.";
      sha256 = optionalNonEmptyString ''
        Pinned ZIP SHA-256 in SRI, raw hex, or `sha256:<hex>` form.
      '';
      repository = optional (types.strMatching "[^/]+/[^/]+") ''
        GitHub repository used when `update_policy = "latest"`.
      '';
      asset_template = optionalNonEmptyString ''
        Latest-release asset name containing a `{tag}` placeholder.
      '';
      archive_root = optionalString "Path inside the release archive to install.";
      manifest_key = optionalString "Manifest public key for a stable unpacked ID; must match id.";
      load_unpacked =
        required types.bool ''
          Whether to load the installed extension as an unpacked extension.
        ''
        // {
          default = false;
        };
    }
  );

  gitExtension = freeformSubmodule (
    namedExtension
    // {
      provider =
        optional
          (types.enum [
            "git"
            "github"
            "gitlab"
            "codeberg"
            "sourcehut"
          ])
          ''
            Hosted Git shorthand, or `git` for an arbitrary URL. May be omitted
            when `url` is set.
          '';
      repository = optionalNonEmptyString ''
        Hosted repository path: `owner/project`, a nested GitLab namespace, or
        `~owner/project` for SourceHut.
      '';
      url = optionalNonEmptyString "Arbitrary Git clone URL for the `git` provider.";
      update_policy = optional (types.enum [
        "latest"
        "pinned"
      ]) "Follow a Git ref or require an exact commit.";
      ref = optionalNonEmptyString ''
        Branch, tag, or other advertised ref. The remote default branch is
        used for latest entries when omitted.
      '';
      commit = optional (types.strMatching "([0-9a-fA-F]{40}|[0-9a-fA-F]{64})") ''
        Full Git object ID required by pinned entries.
      '';
      subdirectory = optionalString "Repository subdirectory exported as the extension root.";
      load_unpacked =
        required types.bool ''
          Whether to load the exported Git tree as an unpacked extension.
        ''
        // {
          default = false;
        };
    }
  );

  networkSettings = freeformSubmodule {
    chrome_version = optional (types.strMatching "[0-9]+(\\.[0-9]+)*") ''
      Chrome product version sent to the Chrome Web Store update service. If
      omitted, use the installed browser's Chromium version, falling back to
      the latest Stable Chrome version when the browser cannot be queried.
    '';
    user_agent = optionalNonEmptyString ''
      User agent used for extension HTTP and GitHub requests. If omitted, read
      the installed browser's native user agent verbatim. If the browser cannot
      be queried, derive it from chrome_version when available, or use the
      configurator's own identity.
    '';
    headers = optional (types.attrsOf types.str) "Additional HTTP request headers.";
    timeout_seconds = optionalUnsigned "HTTP request timeout in seconds.";
    retry_max = optionalUnsigned "Maximum transient HTTP retries; zero disables retries.";
    retry_wait_min_milliseconds = optionalUnsigned "Minimum retry delay in milliseconds.";
    retry_wait_max_milliseconds = optionalUnsigned "Maximum retry delay in milliseconds.";
  };

  extensionSettings = freeformSubmodule {
    chrome_store_update_url = optionalString "Chrome Web Store update service URL.";
    network = optional networkSettings ''
      HTTP identity, Chrome version, headers, timeouts, and retry policy.
    '';
    chrome_store = optional (types.listOf chromeStoreExtension) ''
      Extensions installed from the Chrome Web Store.
    '';
    update_url = optional (types.listOf updateUrlExtension) ''
      Extensions installed through update manifests.
    '';
    crx = optional (types.listOf downloadedExtension) "Checksum-pinned CRX extensions.";
    zip = optional (types.listOf zipExtension) ''
      Latest or checksum-pinned unpacked ZIP extensions.
    '';
    git = optional (types.listOf gitExtension) ''
      Git-backed unpacked extensions from GitHub, GitLab, Codeberg, SourceHut,
      or an arbitrary Git remote.
    '';
  };

  extensionRuntime = freeformSubmodule {
    id = required schema.extensionId "Catalog ID of the extension to configure.";
    page = required types.nonEmptyStr ''
      Relative extension page URL in which to execute the script.
    '';
    script = required pathString ''
      JavaScript function body that may await extension APIs. It receives
      input.files as decoded JSON, input.values as runtime extension_values,
      and input.id as the installed extension ID. The script must be
      idempotent and await all writes before returning.
    '';
    files = optional (types.attrsOf pathString) "Named JSON input files passed to the script.";
    extension_dir = optional pathString ''
      Unpacked extension directory. Defaults to the extension installed by
      the ZIP or Git catalog under the browser installation root.
    '';
    timeout_seconds = optionalUnsigned ''
      Maximum duration for this runtime operation. Zero or omission uses
      180 seconds.
    '';
  };

  extensionStorageSettings = freeformSubmodule {
    files = optional (types.listOf pathString) ''
      Ordered JSON extension-settings documents. Relative strings resolve from
      the generated TOML file; Nix paths are written as absolute store paths.
    '';
    runtime = optional (types.listOf extensionRuntime) ''
      Ordered scripts run in isolated extension pages before normal browser
      use. Successful runs install the extension's local, sync, and IndexedDB
      storage after the temporary browser exits. The target browser must be
      closed. Requires a browser that supports loading unpacked extensions.
    '';
  };
in
{
  extensions = optional extensionSettings "Optional extension installation catalog.";
  extension_settings = optional extensionStorageSettings ''
    Direct storage mutations and scripts used to configure persistent
    extension data.
  '';
}
