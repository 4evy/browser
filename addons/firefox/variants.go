package firefox

import "strings"

type variant struct{ name, linuxLauncher, macApp, macLauncher string }

// Presets provide executable identities only. Profile paths remain explicit
// because Firefox uses random profile names and forks can change their roots
// Unknown variants use the same engine with explicitly configured app paths
var variants = map[string]variant{
	"firefox":   {"Firefox", "firefox", "Firefox", "firefox"},
	"beta":      {"Firefox Beta", "firefox", "Firefox", "firefox"},
	"esr":       {"Firefox ESR", "firefox", "Firefox", "firefox"},
	"developer": {"Firefox Developer Edition", "firefox-devedition", "Firefox Developer Edition", "firefox"},
	"nightly":   {"Firefox Nightly", "firefox-nightly", "Firefox Nightly", "firefox"},
	"librewolf": {"LibreWolf", "librewolf", "LibreWolf", "librewolf"},
	"waterfox":  {"Waterfox", "waterfox", "Waterfox", "waterfox"},
	"floorp":    {"Floorp", "floorp", "Floorp", "floorp"},
	"zen":       {"Zen", "zen", "Zen", "zen"},
}

func (config Config) defaults() Config {
	if config.Variant == "" && len(config.Providers) == 1 {
		config.Variant = config.Providers[0].Variant()
	}
	if config.Variant == "" {
		config.Variant = "firefox"
	}
	preset, known := variants[config.Variant]
	if config.Name == "" {
		config.Name = preset.name
		if !known {
			config.Name = config.Variant
		}
	}
	if config.ExecutableName == "" {
		config.ExecutableName = config.Variant + "-configured"
	}
	if config.Linux.LauncherName == "" {
		config.Linux.LauncherName = preset.linuxLauncher
	}
	if config.MacOS.AppDir == "" && known {
		config.MacOS.AppDir = "/Applications/" + preset.macApp + ".app"
	}
	if config.MacOS.LauncherPath == "" && known {
		config.MacOS.LauncherPath = "Contents/MacOS/" + preset.macLauncher
	}
	return config
}

// Variants lists the bundled identity presets. Explicit paths also support
// unlisted Gecko forks without requiring a new provider or engine
func Variants() []string {
	return strings.Fields("firefox beta esr developer nightly librewolf waterfox floorp zen")
}
