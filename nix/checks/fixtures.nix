{
  browserPackage,
  pkgs,
}:

let
  sampleExtensionSettings = pkgs.writeText "browser-extension-settings.json" (
    builtins.toJSON {
      schema_version = 1;
      local = [
        {
          id = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
          values.module_generated = true;
        }
      ];
    }
  );

  sampleSettings = {
    browser = {
      name = "Example Browser";
      executable_name = "example-browser";
      flags_file = "example-browser-flags.conf";
      flags = [ "--no-first-run" ];
      user_agent = "Example Browser/1.0";

      linux = {
        desktop_id = "example-browser";
        portal_app_id = "org.chromium.Chromium";
      };

      paths.${if pkgs.stdenv.hostPlatform.isDarwin then "macos" else "linux"} = {
        profile_dir = "\${home}/.example-browser/Default";
        external_extension_dirs = [ "\${home}/.example-browser/External Extensions" ];
      };

      preferences = {
        values = [
          {
            path = "browser.example.enabled";
            value = true;
          }
        ];
        accelerators = [
          {
            path = "helium.browser.custom_accelerators";
            command_id = "34014";
            accelerator = "Control+Shift+KeyY";
          }
        ];
        cookies = {
          default = "session_only";
          third_party = "block";
          allow = [ "[*.]example.com" ];
          block = [ "[*.]tracker.example" ];
          session_only = [ "[*.]session.example" ];
        };
      };

      helium = {
        crash_reporting = "ask";
        appearance.layout = "dynamic";
        behavior.new_tab_next_to_active = true;
        behavior.suppress_default_browser_prompt = true;
        privacy.global_privacy_control = true;
        toolbar.show_dynamic_new_tab_button = true;
        services = {
          enabled = true;
          user_consented = true;
          extension_proxy = true;
          ublock_assets = true;
        };
      };

      brave = {
        origin = true;
        tabs = {
          vertical = true;
          hover_mode = "card";
          always_use_mini_accent_icon = true;
        };
        toolbar.web_view_rounded_corners = true;
        behavior.cycle_tabs_by_most_recent_use = true;
        behavior.show_default_browser_prompt = false;
        sidebar.show = "mouseover";
        shields.adblock_only_mode = false;
        profile_values = [
          {
            path = "brave.example.profile_override";
            value = "configured";
          }
        ];
        local_state_values = [
          {
            path = "brave.example.local_state_override";
            value = 7;
          }
        ];
        managed_policies = {
          BrowserSignin = 0;
        };
      };
    };

    extensions = {
      chrome_store_update_url = "https://clients2.google.com/service/update2/crx";
      network = {
        chrome_version = "152.0.7971.0";
        user_agent = "Example Downloader/1.0";
        retry_max = 0;
      };
      chrome_store = [
        {
          id = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
          name = "Example extension";
        }
      ];
      zip = [
        {
          id = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";
          name = "Pinned example";
          update_policy = "pinned";
          version = "1.2.3";
          url = "https://example.test/extension-1.2.3.zip";
          sha256 = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
          archive_root = "extension";
          load_unpacked = true;
        }
      ];
      git = [
        {
          id = "cccccccccccccccccccccccccccccccc";
          name = "Pinned Codeberg example";
          provider = "codeberg";
          repository = "owner/extension";
          update_policy = "pinned";
          ref = "v1.2.3";
          commit = "0123456789abcdef0123456789abcdef01234567";
          subdirectory = "dist/extension";
          load_unpacked = true;
        }
      ];
    };

    extension_settings.files = [ sampleExtensionSettings ];
  };

  namedSettings = {
    browser = {
      name = "Second Browser";
      executable_name = "second-browser";
    };
  };
in
{
  inherit
    namedSettings
    sampleSettings
    ;

  moduleConfiguration.programs.browser = {
    enable = true;
    package = browserPackage;
    settings = sampleSettings;
    configurations.second = namedSettings;
  };

  homeBase.home = {
    username = "browser-test";
    homeDirectory =
      if pkgs.stdenv.hostPlatform.isDarwin then "/Users/browser-test" else "/home/browser-test";
    stateVersion = "25.11";
  };

  # Keep module checks focused; option documentation has its own check.
  homeBase.manual.manpages.enable = false;
}
