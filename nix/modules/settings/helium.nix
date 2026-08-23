{ schema }:

let
  inherit (schema)
    optional
    optionalBool
    optionalString
    optionalSubmodule
    types
    ;
in
{
  completed_onboarding = optionalBool "Whether Helium's onboarding has been completed.";
  crash_reporting = optional (types.enum [
    "disabled"
    "ask"
    "automatic"
  ]) "Helium crash-report upload mode.";

  services = optionalSubmodule {
    enabled = optionalBool "Allow Helium services globally.";
    user_consented = optionalBool "Record the user's consent to Helium services.";
    origin_override = optionalString "Custom HTTPS or localhost Helium services origin.";
    extension_proxy = optionalBool "Use Helium's privacy-preserving extension proxy.";
    bangs = optionalBool "Fetch and enable Helium's native search bangs.";
    spellcheck_files = optionalBool "Allow Helium to fetch spellcheck dictionaries.";
    browser_updates = optionalBool "Allow Helium browser or component update checks.";
    ublock_assets = optionalBool "Allow Helium to update its bundled uBlock assets.";
  } "Helium-only service controls.";

  appearance = optionalSubmodule {
    layout = optional (types.enum [
      "classic"
      "compact"
      "vertical"
      "dynamic"
    ]) "Helium browser layout.";
    vertical_right_aligned = optionalBool "Place Helium vertical tabs on the right.";
    centered_location_bar = optionalBool "Center Helium's location bar.";
    minimal_location_bar = optionalBool "Use Helium's minimal location bar.";
    rounded_frame = optionalBool "Round Helium's web-content frame.";
    native_frame_materials = optionalBool "Use native frame materials where supported.";
    zen_mode = optionalBool "Enable Helium zen mode.";
    zen_mode_sidebar_pinned = optionalBool "Keep the zen-mode sidebar visible.";
    zen_mode_top_chrome_pinned = optionalBool "Keep top chrome visible in zen mode.";
  } "Helium-only appearance controls.";

  behavior = optionalSubmodule {
    new_tab_next_to_active = optionalBool "Open new tabs next to the active tab.";
    cycle_tabs_by_most_recent_use = optionalBool "Cycle tabs in most-recently-used order.";
    shift_right_click_menu = optionalBool "Enable Helium's Shift-right-click menu behavior.";
    copy_page_url_shortcut = optionalBool "Enable Helium's copy-page-URL shortcut.";
    vertical_collapse_shortcut = optionalBool "Enable Helium's vertical-tab collapse shortcut.";
    suppress_default_browser_prompt = optionalBool "Suppress Helium's default-browser prompt.";
  } "Helium-only tab and shortcut behavior.";

  privacy = optionalSubmodule {
    global_privacy_control = optionalBool "Send the Global Privacy Control signal.";
    noise = optionalBool "Enable Helium's anti-fingerprinting noise.";
  } "Helium-only privacy controls.";

  toolbar = optionalSubmodule {
    show_back_button = optionalBool "Show Helium's back button.";
    show_reload_button = optionalBool "Show Helium's reload button.";
    show_avatar_button = optionalBool "Show Helium's avatar button.";
    show_extensions_button = optionalBool "Show Helium's extensions button.";
    show_menu_button = optionalBool "Show Helium's app-menu button.";
    show_media_button = optionalBool "Show Helium's media button.";
    show_vertical_tabs_collapse_button = optionalBool "Show the vertical-tabs collapse button.";
    show_dynamic_new_tab_button = optionalBool "Show Helium's dynamic new-tab button.";
    show_page_zoom_indicator = optionalBool "Show Helium's page-zoom indicator.";
  } "Helium-only toolbar controls.";
}
