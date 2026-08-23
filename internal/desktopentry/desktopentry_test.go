package desktopentry

import (
	"testing"

	"gopkg.in/ini.v1"
)

func TestRewrite(t *testing.T) {
	text, err := RewriteWithIcon(
		"[Desktop Entry]\nExec=example-browser --new-window %U\n",
		"/home/test/My $Browser/browser",
		"example-browser",
		"ExampleBrowser",
		"example-browser",
	)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := ini.Load([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	desktop := entry.Section("Desktop Entry")
	if got, want := desktop.Key("Exec").String(),
		`"/home/test/My \\$Browser/browser" --new-window %U`; got != want {
		t.Fatalf("desktop Exec = %q, want %q", got, want)
	}
	if got := desktop.Key("StartupNotify").String(); got != "false" {
		t.Fatalf("desktop StartupNotify = %q", got)
	}
	if got := desktop.Key("StartupWMClass").String(); got != "ExampleBrowser" {
		t.Fatalf("desktop StartupWMClass = %q", got)
	}
	if got := desktop.Key("Icon").String(); got != "example-browser" {
		t.Fatalf("desktop Icon = %q", got)
	}
}

func TestRewriteLeavesIconAloneWhenNotConfigured(t *testing.T) {
	text, err := Rewrite(
		"[Desktop Entry]\nExec=example-browser\nIcon=bundled-icon\n",
		"/usr/bin/example-browser",
		"example-browser",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := ini.Load([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	if got := entry.Section("Desktop Entry").Key("Icon").String(); got != "bundled-icon" {
		t.Fatalf("desktop Icon = %q", got)
	}
}

func TestEscapeExecutable(t *testing.T) {
	tests := []struct {
		executable string
		want       string
	}{
		{executable: "/usr/bin/browser", want: "/usr/bin/browser"},
		{executable: "/opt/My Browser", want: `"/opt/My Browser"`},
		{executable: "/opt/$browser", want: `"/opt/\\$browser"`},
		{executable: `/opt/\browser`, want: `"/opt/\\\\browser"`},
		{executable: "/opt/%browser", want: `"/opt/%%browser"`},
	}
	for _, test := range tests {
		got, err := EscapeExecutable(test.executable)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Errorf("quoted executable %q = %q, want %q", test.executable, got, test.want)
		}
	}
}

func TestAlias(t *testing.T) {
	text, err := Alias("[Desktop Entry]\nExec=example-browser\n")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := ini.Load([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	if got := entry.Section("Desktop Entry").Key("NoDisplay").String(); got != "true" {
		t.Fatalf("desktop NoDisplay = %q, want true", got)
	}
}

func TestRewriteRejectsEqualsInExecutable(t *testing.T) {
	_, err := Rewrite(
		"[Desktop Entry]\nExec=example-browser\n",
		"/home/test/browser=invalid",
		"example-browser",
		"",
	)
	if err == nil {
		t.Fatal("expected invalid desktop entry executable")
	}
}
