package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func run() error {
	if len(os.Args) != 7 {
		return fmt.Errorf("usage: nixos LAUNCHER_FLAGS BROWSER FAKE_BUNDLE ICON EXPECTED_ARGUMENTS LAUNCHER_CHECK")
	}
	flags, browser, bundle := quote(os.Args[1]), quote(os.Args[2]), quote(os.Args[3])
	icon, arguments, launcherCheck := quote(os.Args[4]), quote(os.Args[5]), quote(os.Args[6])
	userEnv := "runuser -u alice -- env HOME=/home/alice XDG_CONFIG_HOME=/home/alice/.config "
	checks := []struct {
		name     string
		commands []string
	}{
		{
			"the NixOS module installs the package and generated files",
			[]string{
				"test -x /run/current-system/sw/bin/tweezle",
				`grep -F 'executable_name = "vm-browser"' /etc/tweezle/tweezle.toml`,
				`grep -F 'executable_name = "secondary-vm-browser"' /etc/tweezle/secondary.toml`,
			},
		},
		{
			"a normal user configures the fake browser bundle",
			[]string{
				"runuser -u alice -- mkdir -p /home/alice/.config/tweezle",
				"install -m644 -o alice -g users " + flags + " /home/alice/.config/tweezle/vm-browser-flags.conf",
				userEnv + "XDG_DATA_HOME=/home/alice/.local/share " + browser +
					" apply /etc/tweezle/tweezle.toml --platform=linux" +
					" --install-dir=/home/alice/.local/share/tweezle/vm-browser" +
					" --app-dir=" + bundle + "/app --bin-dir=/home/alice/.local/bin -- --from-command",
			},
		},
		{
			"the launcher and its alias are native executable links",
			[]string{
				"test -L /home/alice/.local/bin/vm-browser",
				`test "$(readlink /home/alice/.local/bin/vm-browser)" = ` + browser,
				`test "$(readlink /home/alice/.local/bin/vm-browser-alias)" = vm-browser`,
				"jq -e --arg browser " + bundle + "/app/fake-browser -f " + launcherCheck +
					" /home/alice/.local/bin/vm-browser.tweezle-launcher.json",
			},
		},
		{
			"profile, desktop, and icon integration is applied",
			[]string{
				"jq -e '.vm.checked == true' /home/alice/.config/vm-browser/Default/Preferences",
				"grep -E '^Exec[[:space:]]*=[[:space:]]*/home/alice/.local/bin/vm-browser %U$' /home/alice/.local/share/applications/vm-browser.desktop",
				"grep -E '^StartupWMClass[[:space:]]*=[[:space:]]*vm-browser$' /home/alice/.local/share/applications/vm-browser.desktop",
				"grep -E '^Icon[[:space:]]*=[[:space:]]*vm-browser$' /home/alice/.local/share/applications/vm-browser.desktop",
				"grep -E '^NoDisplay[[:space:]]*=[[:space:]]*true$' /home/alice/.local/share/applications/org.chromium.Chromium.desktop",
				"cmp " + icon + " /home/alice/.local/share/icons/hicolor/256x256/apps/vm-browser.png",
			},
		},
		{
			"the generated launcher executes the browser with every flag source",
			[]string{
				userEnv + "/home/alice/.local/bin/vm-browser --runtime > /tmp/vm-browser-arguments",
				"diff -u " + arguments + " /tmp/vm-browser-arguments",
			},
		},
	}
	for _, check := range checks {
		fmt.Println(check.name)
		for _, command := range check.commands {
			cmd := exec.Command("sh", "-eu", "-c", command)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("%s: %s: %w", check.name, command, err)
			}
		}
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
