{ lib, schema }:

let
  metadata = lib.trivial.importJSON ../metadata.json;
  choice = schema.optionalChoice metadata.choices;
  inherit (schema) optionalBool optionalSubmodule;
in
{
  completed_onboarding = optionalBool "Mark the Vivaldi welcome screen as seen.";

  tabs = optionalSubmodule {
    position = choice "tabs.position" "Place the Vivaldi tab bar.";
    stacking = choice "tabs.stacking" "Choose the Vivaldi tab stack layout.";
    new_tab_position = choice "tabs.new_tab_position" "Choose where new tabs open.";
    activate_on_close = choice "tabs.activate_on_close" "Choose which tab activates after closing a tab.";
    cycle_by_recent_order = optionalBool "Cycle tabs in most-recently-used order.";
    open_new_in_background = optionalBool "Open new tabs without taking focus.";
    defer_loading_after_restore = optionalBool "Load restored tabs only when activated.";
    show_close_button = optionalBool "Show tab close buttons.";
    show_thumbnails = optionalBool "Show page thumbnails inside tabs.";
    show_tooltips = optionalBool "Show tab preview tooltips.";
    visible = optionalBool "Show the Vivaldi tab bar.";
  } "Vivaldi tabs controls.";

  appearance = optionalSubmodule {
    native_window = optionalBool "Use native window decorations.";
    status_bar = choice "appearance.status_bar" "Show, hide, or overlay the status bar.";
    force_dark_theme = optionalBool "Force a dark Vivaldi theme.";
  } "Vivaldi appearance controls.";

  panels = optionalSubmodule {
    position = choice "panels.position" "Place the Vivaldi panel bar.";
    floating = optionalBool "Display panels over the webpage.";
    auto_close = optionalBool "Automatically close floating panels.";
    show_toggle = optionalBool "Show the panel toggle along the window edge.";
  } "Vivaldi panels controls.";

  address_bar = optionalSubmodule {
    position = choice "address_bar.position" "Place the Vivaldi address bar.";
    visible = optionalBool "Show the Vivaldi address bar.";
    show_full_url = optionalBool "Show the full URL in the address field.";
    show_progress = optionalBool "Show page loading progress in the address bar.";
    search_suggestions = optionalBool "Allow search suggestions.";
    direct_match = optionalBool "Enable Vivaldi Direct Match suggestions.";
  } "Vivaldi address bar controls.";

  privacy = optionalSubmodule {
    block_pings = optionalBool "Block hyperlink auditing pings.";
    block_abusive_ads = optionalBool "Update and load the abusive advertising block list.";
  } "Vivaldi privacy controls.";

  behavior = optionalSubmodule {
    check_default_browser = optionalBool "Check whether Vivaldi is the default browser on startup.";
    mouse_gestures = optionalBool "Enable Vivaldi mouse gestures.";
    rocker_gestures = optionalBool "Enable Vivaldi rocker gestures.";
    scroll_to_switch_tabs = optionalBool "Switch tabs with the mouse wheel.";
    confirm_window_close = optionalBool "Confirm closing a Vivaldi window.";
  } "Vivaldi behavior controls.";
}
