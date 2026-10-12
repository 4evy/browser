{ schema }:

let
  inherit (schema)
    optional
    optionalBool
    optionalNonEmptyString
    types
    ;
in
{
  disable_beforeunload = optionalBool "Disable JavaScript beforeunload dialogs.";
  disable_grease_tls = optionalBool "Disable GREASE for TLS.";
  disable_search_engine_collection = optionalBool "Disable automatic search engine scraping.";
  canvas_image_data_noise = optionalBool "Add noise to canvas image data returned to JavaScript.";
  canvas_measuretext_noise = optionalBool "Add noise to canvas text measurements.";
  client_rects_noise = optionalBool "Add noise to client rectangle measurements.";
  force_punycode_hostnames = optionalBool "Display internationalized hostnames as punycode.";
  hide_crashed_bubble = optionalBool "Hide the restore-pages prompt after a crash.";
  keep_old_history = optionalBool "Keep local browser history beyond 90 days.";
  popups_to_tabs = optionalBool "Open popups in tabs.";
  disabling_javascript_optimization_disables_jit = optionalBool ''
    Disable JIT when the JavaScript optimization setting is turned off.
  '';
  hide_bookmark_bar_on_new_tab = optionalBool "Hide the bookmark bar on the new tab page.";
  keep_window_with_last_tab = optionalBool "Keep the window open when its last tab closes.";
  enable_incognito_themes = optionalBool "Allow themes in incognito windows.";
  hide_extensions_menu = optionalBool "Hide the extensions container, including pinned extensions.";
  hide_fullscreen_exit_ui = optionalBool "Hide the fullscreen exit button and popup.";
  hide_tab_close_buttons = optionalBool "Hide tab close buttons.";
  remove_grab_handle = optionalBool "Remove the empty tabstrip space reserved for moving the window.";
  custom_new_tab_url = optionalNonEmptyString "Set an internal, external, or local new tab URL.";
  http_accept_header = optionalNonEmptyString "Override the default HTTP Accept header.";
  max_connections_per_host = optional (types.enum [
    6
    15
  ]) "Limit connections per host to 6 or 15.";
  extension_mime_handling = optional (types.enum [
    "download-as-regular-file"
    "always-prompt-for-install"
  ]) "Choose how CRX and user-script downloads are handled.";
  omnibox_autocomplete_filtering = optional (types.enum [
    "search"
    "search-bookmarks"
    "search-chrome"
    "search-bookmarks-chrome"
  ]) "Restrict omnibox autocomplete to selected result sources.";
  close_confirmation = optional (types.enum [
    "last"
    "multiple"
  ]) "Confirm closing the last window with several tabs, or closing when multiple windows are open.";
  scroll_tabs = optional (types.enum [
    "always"
    "never"
    "incognito-and-guest"
  ]) "Choose when scrolling over the tabstrip switches tabs.";
  show_avatar_button = optional (types.enum [
    "always"
    "never"
    "incognito-and-guest"
  ]) "Choose when the avatar button is visible.";
  tab_hover_cards = optional (types.enum [
    "none"
    "tooltip"
  ]) "Hide tab hover cards or replace them with tooltips.";
}
