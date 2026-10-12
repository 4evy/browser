package ungoogled

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/4evy/tweezle/addons/chromium"
)

// Config exposes desktop switches audited against ungoogled Chromium
// 155.0.8059.39, revision ffeb45a2bb88bc56a1dfed31410aeb10e9efcd5a
// Source: https://github.com/ungoogled-software/ungoogled-chromium/blob/ffeb45a2bb88bc56a1dfed31410aeb10e9efcd5a/docs/flags.md
// True emits the corresponding switch; false and nil emit nothing. Empty enums
// and nil values are omitted; configured strings must be nonempty. Shared
// browser.flags can configure Chromium feature lists and additional switches
type Config struct {
	DisableBeforeUnload                *bool                        `toml:"disable_beforeunload"`
	DisableGreaseTLS                   *bool                        `toml:"disable_grease_tls"`
	DisableSearchEngineCollection      *bool                        `toml:"disable_search_engine_collection"`
	CanvasImageDataNoise               *bool                        `toml:"canvas_image_data_noise"`
	CanvasMeasureTextNoise             *bool                        `toml:"canvas_measuretext_noise"`
	ClientRectsNoise                   *bool                        `toml:"client_rects_noise"`
	ForcePunycodeHostnames             *bool                        `toml:"force_punycode_hostnames"`
	HideCrashedBubble                  *bool                        `toml:"hide_crashed_bubble"`
	KeepOldHistory                     *bool                        `toml:"keep_old_history"`
	PopupsToTabs                       *bool                        `toml:"popups_to_tabs"`
	DisablingJSOptimizationDisablesJIT *bool                        `toml:"disabling_javascript_optimization_disables_jit"`
	HideBookmarkBarOnNewTab            *bool                        `toml:"hide_bookmark_bar_on_new_tab"`
	KeepWindowWithLastTab              *bool                        `toml:"keep_window_with_last_tab"`
	EnableIncognitoThemes              *bool                        `toml:"enable_incognito_themes"`
	HideExtensionsMenu                 *bool                        `toml:"hide_extensions_menu"`
	HideFullscreenExitUI               *bool                        `toml:"hide_fullscreen_exit_ui"`
	HideTabCloseButtons                *bool                        `toml:"hide_tab_close_buttons"`
	RemoveGrabHandle                   *bool                        `toml:"remove_grab_handle"`
	CustomNewTabURL                    *string                      `toml:"custom_new_tab_url"`
	HTTPAcceptHeader                   *string                      `toml:"http_accept_header"`
	MaxConnectionsPerHost              *int                         `toml:"max_connections_per_host"`
	ExtensionMIMEHandling              ExtensionMIMEHandling        `toml:"extension_mime_handling"`
	OmniboxAutocompleteFiltering       OmniboxAutocompleteFiltering `toml:"omnibox_autocomplete_filtering"`
	CloseConfirmation                  CloseConfirmation            `toml:"close_confirmation"`
	ScrollTabs                         ScrollTabs                   `toml:"scroll_tabs"`
	ShowAvatarButton                   ShowAvatarButton             `toml:"show_avatar_button"`
	TabHoverCards                      TabHoverCards                `toml:"tab_hover_cards"`
}

type ExtensionMIMEHandling string

const (
	ExtensionMIMEDownload ExtensionMIMEHandling = "download-as-regular-file"
	ExtensionMIMEPrompt   ExtensionMIMEHandling = "always-prompt-for-install"
)

type OmniboxAutocompleteFiltering string

const (
	OmniboxSearch                OmniboxAutocompleteFiltering = "search"
	OmniboxSearchBookmarks       OmniboxAutocompleteFiltering = "search-bookmarks"
	OmniboxSearchChrome          OmniboxAutocompleteFiltering = "search-chrome"
	OmniboxSearchBookmarksChrome OmniboxAutocompleteFiltering = "search-bookmarks-chrome"
)

type CloseConfirmation string

const (
	CloseConfirmationLast     CloseConfirmation = "last"
	CloseConfirmationMultiple CloseConfirmation = "multiple"
)

type ScrollTabs string

const (
	ScrollTabsAlways            ScrollTabs = "always"
	ScrollTabsNever             ScrollTabs = "never"
	ScrollTabsIncognitoAndGuest ScrollTabs = "incognito-and-guest"
)

type ShowAvatarButton string

const (
	ShowAvatarAlways            ShowAvatarButton = "always"
	ShowAvatarNever             ShowAvatarButton = "never"
	ShowAvatarIncognitoAndGuest ShowAvatarButton = "incognito-and-guest"
)

type TabHoverCards string

const (
	TabHoverCardsNone    TabHoverCards = "none"
	TabHoverCardsTooltip TabHoverCards = "tooltip"
)

type choice struct {
	name       string
	switchName string
	value      string
	options    []string
}

func (config Config) choices() []choice {
	return []choice{
		{
			"extension_mime_handling", "extension-mime-request-handling",
			string(config.ExtensionMIMEHandling),
			[]string{string(ExtensionMIMEDownload), string(ExtensionMIMEPrompt)},
		},
		{
			"omnibox_autocomplete_filtering", "omnibox-autocomplete-filtering",
			string(config.OmniboxAutocompleteFiltering),
			[]string{
				string(OmniboxSearch), string(OmniboxSearchBookmarks),
				string(OmniboxSearchChrome), string(OmniboxSearchBookmarksChrome),
			},
		},
		{
			"close_confirmation", "close-confirmation",
			string(config.CloseConfirmation),
			[]string{string(CloseConfirmationLast), string(CloseConfirmationMultiple)},
		},
		{
			"scroll_tabs", "scroll-tabs", string(config.ScrollTabs),
			[]string{string(ScrollTabsAlways), string(ScrollTabsNever), string(ScrollTabsIncognitoAndGuest)},
		},
		{
			"show_avatar_button", "show-avatar-button",
			string(config.ShowAvatarButton),
			[]string{string(ShowAvatarAlways), string(ShowAvatarNever), string(ShowAvatarIncognitoAndGuest)},
		},
		{
			"tab_hover_cards", "tab-hover-cards", string(config.TabHoverCards),
			[]string{string(TabHoverCardsNone), string(TabHoverCardsTooltip)},
		},
	}
}

func (config Config) Validate() error {
	var errs []error
	for _, choice := range config.choices() {
		if choice.value != "" && !slices.Contains(choice.options, choice.value) {
			errs = append(errs, fmt.Errorf(
				"browser.ungoogled.%s must be one of %s, got %q",
				choice.name, strings.Join(choice.options, ", "), choice.value,
			))
		}
	}
	if value := config.MaxConnectionsPerHost; value != nil && *value != 6 && *value != 15 {
		errs = append(errs, errors.New("browser.ungoogled.max_connections_per_host must be 6 or 15"))
	}
	for name, value := range map[string]*string{
		"custom_new_tab_url": config.CustomNewTabURL,
		"http_accept_header": config.HTTPAcceptHeader,
	} {
		if value != nil && (strings.TrimSpace(*value) == "" || strings.ContainsAny(*value, "\x00\r\n")) {
			errs = append(errs, fmt.Errorf(
				"browser.ungoogled.%s must be nonempty and contain no NUL or line breaks",
				name,
			))
		}
	}
	return errors.Join(errs...)
}

// Contributions emits switches rather than writing chrome://flags state
// Source: patches/core/ungoogled-chromium/extensions-manifestv2.patch at the
// audited revision retains Manifest V2 support, including unpacked extensions
func (config Config) Contributions() chromium.Contribution {
	contribution := chromium.Contribution{SupportsManifestV2: new(true)}
	for _, option := range []struct {
		value *bool
		flag  string
	}{
		{config.DisableBeforeUnload, "--disable-beforeunload"},
		{config.DisableGreaseTLS, "--disable-grease-tls"},
		{config.DisableSearchEngineCollection, "--disable-search-engine-collection"},
		{config.CanvasImageDataNoise, "--fingerprinting-canvas-image-data-noise"},
		{config.CanvasMeasureTextNoise, "--fingerprinting-canvas-measuretext-noise"},
		{config.ClientRectsNoise, "--fingerprinting-client-rects-noise"},
		{config.ForcePunycodeHostnames, "--force-punycode-hostnames"},
		{config.HideCrashedBubble, "--hide-crashed-bubble"},
		{config.KeepOldHistory, "--keep-old-history"},
		{config.PopupsToTabs, "--popups-to-tabs"},
		{config.DisablingJSOptimizationDisablesJIT, "--disabling-javascript-optimization-disables-jit"},
		{config.HideBookmarkBarOnNewTab, "--bookmark-bar-ntp=never"},
		{config.KeepWindowWithLastTab, "--close-window-with-last-tab=never"},
		{config.EnableIncognitoThemes, "--enable-incognito-themes"},
		{config.HideExtensionsMenu, "--hide-extensions-menu"},
		{config.HideFullscreenExitUI, "--hide-fullscreen-exit-ui"},
		{config.HideTabCloseButtons, "--hide-tab-close-buttons"},
		{config.RemoveGrabHandle, "--remove-grab-handle"},
	} {
		if option.value != nil && *option.value {
			contribution.Flags = append(contribution.Flags, option.flag)
		}
	}
	for _, choice := range config.choices() {
		if choice.value != "" {
			contribution.Flags = append(contribution.Flags, "--"+choice.switchName+"="+choice.value)
		}
	}
	if config.CustomNewTabURL != nil {
		contribution.Flags = append(contribution.Flags, "--custom-ntp="+*config.CustomNewTabURL)
	}
	if config.HTTPAcceptHeader != nil {
		contribution.Flags = append(contribution.Flags, "--http-accept-header="+*config.HTTPAcceptHeader)
	}
	if config.MaxConnectionsPerHost != nil {
		contribution.Flags = append(contribution.Flags, "--max-connections-per-host="+strconv.Itoa(*config.MaxConnectionsPerHost))
	}
	return contribution
}

var _ chromium.Provider = Config{}
