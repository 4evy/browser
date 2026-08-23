{ schema }:

let
  inherit (schema)
    freeformSubmodule
    optional
    optionalBool
    optionalInt
    optionalString
    optionalSubmodule
    required
    types
    ;

  preferenceValue = freeformSubmodule {
    path = required types.nonEmptyStr "Dotted Brave preference path to update.";
    value = required types.toml "TOML value to write at `path`.";
  };
in
{
  origin = optionalBool ''
    Reproduce Brave Origin's disabled-service policies and branded UI defaults
    on regular Brave without changing purchase, SKU, or branding state.
    Explicit `features` and UI settings override this preset.
  '';

  disable_web3 = optionalBool ''
    Disable Brave Wallet, Rewards/BAT, decentralized DNS, and legacy IPFS and
    WebTorrent integration. Explicit `features` values override this preset.
  '';

  disable_annoyances = optionalBool ''
    Disable Web3 plus Brave ads, sponsored content, AI, VPN, News, Talk,
    Playlist, telemetry, promotions, and other bundled service surfaces.
    Explicit `features`, raw values, and managed policies override this preset.
  '';

  tabs = optionalSubmodule {
    hover_mode = optional (types.enum [
      "tooltip"
      "card"
      "card_with_preview"
    ]) "Brave tab-hover presentation.";
    vertical = optionalBool "Enable Brave vertical tabs.";
    collapsed = optionalBool "Collapse Brave vertical tabs.";
    expanded_state_per_window = optionalBool "Remember vertical-tab expansion per window.";
    show_window_title = optionalBool "Show the window title with vertical tabs.";
    hide_completely_when_collapsed = optionalBool "Fully hide collapsed vertical tabs.";
    floating = optionalBool "Use Brave's floating vertical tabs.";
    show_toggle_button = optionalBool "Show the vertical-tabs toggle.";
    expanded_width = optionalInt "Expanded vertical-tab width in pixels.";
    on_right = optionalBool "Place vertical tabs on the right.";
    show_scrollbar = optionalBool "Show the vertical-tab scrollbar.";
    tree = optionalBool "Enable Brave tree tabs.";
    shared_pinned = optionalBool "Share pinned tabs between windows.";
    always_hide_close_button = optionalBool "Always hide tab close buttons.";
    middle_click_close = optionalBool "Close a tab when it is middle-clicked.";
    disable_clickable_mute_indicator = optionalBool "Prevent clicks on the tab mute indicator.";
    min_width = optional (types.enum [
      "default"
      "minimum"
      "medium"
      "large"
      "full"
    ]) "Brave horizontal-tab minimum-width mode.";
    scrollable_horizontal = optionalBool "Use Brave's scrollable horizontal tab strip.";
    show_horizontal_scroll_buttons = optionalBool "Show horizontal tab-strip scroll buttons.";
    always_use_mini_accent_icon = optionalBool "Always use Brave's mini accent tab icon.";
    compact_horizontal = optionalBool "Use Brave's compact horizontal tab layout.";
  } "Brave-only tab controls.";

  toolbar = optionalSubmodule {
    location_bar_wide = optionalBool "Use Brave's wide location bar.";
    web_view_rounded_corners = optionalBool "Round Brave web-view corners.";
    subtle_app_menu_logo = optionalBool "Use Brave's subtle app-menu logo.";
    show_bookmarks_button = optionalBool "Show Brave's bookmarks toolbar button.";
    show_side_panel_button = optionalBool "Show Brave's side-panel toolbar button.";
    show_screenshot_button = optionalBool "Show Brave's screenshot toolbar button.";
  } "Brave-only toolbar and frame controls.";

  behavior = optionalSubmodule {
    cycle_tabs_by_most_recent_use = optionalBool "Cycle Brave tabs in most-recently-used order.";
    confirm_window_close = optionalBool "Ask for confirmation before closing a Brave window.";
    close_window_with_last_tab = optionalBool "Close the Brave window when its last tab closes.";
    show_fullscreen_reminder = optionalBool "Show Brave's fullscreen reminder.";
    show_default_browser_prompt = optionalBool "Show Brave's default-browser prompt.";
  } "Brave-only window and tab behavior.";

  sidebar = optionalSubmodule {
    show = optional (types.enum [
      "always"
      "mouseover"
      "never"
    ]) "When Brave's sidebar is shown.";
  } "Brave-only sidebar controls.";

  shields = optionalSubmodule {
    adblock_only_mode = optionalBool "Use Brave Shields in ad-block-only mode.";
    custom_filters = optionalString "Brave ad-block custom filter rules.";
    facebook_embeds = optionalBool "Allow Facebook embeds through Brave Shields.";
    twitter_embeds = optionalBool "Allow Twitter embeds through Brave Shields.";
    linkedin_embeds = optionalBool "Allow LinkedIn embeds through Brave Shields.";
  } "Brave Shields controls stored in Local State.";

  features =
    optionalSubmodule
      {
        wallet = optionalBool "Enable Brave Wallet and its provider injection.";
        rewards = optionalBool "Enable Brave Rewards/BAT and its browser surfaces.";
        decentralized_dns = optionalBool "Enable ENS, SNS, and Unstoppable Domains resolution.";
        ipfs = optionalBool "Enable the deprecated IPFS compatibility policy.";
        webtorrent = optionalBool "Enable the deprecated WebTorrent preference.";
        crypto_widgets = optionalBool "Enable legacy FTX, Crypto.com, Gemini, and Binance widgets.";
        ads = optionalBool "Enable Brave Ads, including search-result and notification ads.";
        sponsored_content = optionalBool "Enable sponsored new-tab backgrounds and sites.";
        ai_chat = optionalBool "Enable Brave Leo AI and its browser entry points.";
        local_ai = optionalBool "Enable Brave local AI and history embeddings.";
        vpn = optionalBool "Enable Brave VPN surfaces and Smart Proxy Routing.";
        news = optionalBool "Enable Brave News and its new-tab and toolbar surfaces.";
        talk = optionalBool "Enable Brave Talk and its new-tab surface.";
        playlist = optionalBool "Enable Brave Playlist.";
        web_discovery = optionalBool "Enable Brave Web Discovery contributions.";
        p3a = optionalBool "Enable Brave Privacy-Preserving Product Analytics.";
        stats = optionalBool "Enable Brave anonymous usage/statistics pings.";
        email_aliases = optionalBool "Enable Brave Email Aliases and autofill suggestions.";
        search_promotions = optionalBool "Enable Brave Search conversion prompts.";
        suggested_sites = optionalBool "Enable Brave suggested-site suggestions.";
        new_tab_widgets = optionalBool "Enable Brave's bundled new-tab widgets.";
        speedreader = optionalBool "Enable Brave Speedreader.";
        wayback_machine = optionalBool "Enable Brave's Wayback Machine integration.";
        psst = optionalBool "Enable Brave's PSST integration.";
        tor = optionalBool "Enable private windows with Tor.";
        crash_reporting_prompt = optionalBool "Allow Brave to prompt for crash reporting.";
      }
      ''
        Granular Brave feature controls. These values take precedence over the
        presets and update both user preferences and available managed policies.
      '';

  profile_values = optional (types.listOf preferenceValue) ''
    Arbitrary Brave profile Preferences applied after every typed setting.
    Use this escape hatch for any current or future Brave preference.
  '';

  local_state_values = optional (types.listOf preferenceValue) ''
    Arbitrary Brave Local State values applied after every typed setting.
  '';

  managed_policies = optional (types.attrsOf types.toml) ''
    Arbitrary Chromium enterprise policies applied after generated Brave
    feature policies. Render these with `browser policy render`.
  '';
}
