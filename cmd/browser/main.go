package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"

	browser "github.com/4evy/browser"
	"github.com/alecthomas/kong"
)

const (
	applicationName   = "browser"
	defaultVersion    = "dev"
	standardInputPath = "-"
)

var version = defaultVersion

type commandLine struct {
	Configure      configureCommand      `cmd:"" help:"Install extensions and create a browser launcher."`
	ApplyProfile   applyProfileCommand   `cmd:"" name:"apply-profile-settings" help:"Apply browser and extension profile settings."`
	ApplyExtension applyExtensionCommand `cmd:"" name:"apply-extension-settings" help:"Apply extension storage settings."`
	VersionCommand versionCommand        `cmd:"" name:"version" help:"Print the browser version."`
	Version        kong.VersionFlag      `name:"version" short:"v" help:"Print the browser version and exit."`
}

type settingsCommand struct {
	ConfigPath string   `name:"config" default:"browser.toml" help:"TOML configuration file."`
	InputPath  string   `name:"input" help:"JSON input file (- for standard input)."`
	Settings   []string `sep:"none" help:"Extension settings JSON file (repeatable)."`
}

type configureCommand struct {
	settingsCommand `embed:""`
	Mode            browser.Mode `required:"" enum:"macos,linux" help:"Installation mode."`
	Root            string       `required:"" help:"Cache and extension installation directory."`
	AppDir          string       `name:"app-dir" help:"Existing browser application directory."`
	BinDir          string       `name:"bin-dir" required:"" help:"Launcher output directory."`
	BrowserFlags    string       `name:"flags" help:"Shell-quoted browser flags."`
	ApplySettings   bool         `default:"true" negatable:"" help:"Apply profile settings after configuring."`
}

type applyCommand struct {
	settingsCommand `embed:""`
	ProfileDir      string `name:"profile-dir" required:"" help:"Chromium profile directory."`
}

type applyProfileCommand struct {
	applyCommand `embed:""`
}

type applyExtensionCommand struct {
	applyCommand `embed:""`
}

type versionCommand struct{}

func main() {
	if handled, err := browser.RunLauncher(os.Args[0], os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, filepath.Base(os.Args[0])+":", err)
			os.Exit(1)
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, applicationName+":", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string) error {
	var cli commandLine
	parser, err := kong.New(
		&cli,
		kong.Name(applicationName),
		kong.Description("Configure a Chromium-family browser."),
		kong.Vars{"version": version},
		kong.UsageOnError(),
		kong.BindFor(ctx),
	)
	if err != nil {
		return err
	}
	context, err := parser.Parse(arguments)
	if err != nil {
		return err
	}
	return context.Run()
}

func (options *configureCommand) Run(ctx context.Context) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find browser executable: %w", err)
	}
	config, input, err := options.loadConfigAndInput()
	if err != nil {
		return err
	}
	return browser.Configure(ctx, browser.ConfigureOptions{
		Config:             config,
		Mode:               options.Mode,
		Root:               options.Root,
		AppDir:             options.AppDir,
		BinDir:             options.BinDir,
		Flags:              options.BrowserFlags,
		Settings:           options.Settings,
		Input:              input,
		ApplySettings:      options.ApplySettings,
		LauncherExecutable: executable,
	})
}

func (options *applyProfileCommand) Run(ctx context.Context) error {
	return options.apply(ctx, browser.Browser.ApplyProfileSettings)
}

func (options *applyExtensionCommand) Run(ctx context.Context) error {
	return options.apply(ctx, browser.Browser.ApplyExtensionSettings)
}

type applySettingsFunc func(browser.Browser, context.Context, browser.ApplyOptions) error

func (options *applyCommand) apply(ctx context.Context, apply applySettingsFunc) error {
	config, input, err := options.loadConfigAndInput()
	if err != nil {
		return err
	}
	instance, err := browser.New(config)
	if err != nil {
		return err
	}
	applyOptions := browser.ApplyOptions{
		ProfileDir: options.ProfileDir,
		Settings:   options.Settings,
		Input:      input,
	}
	return apply(instance, ctx, applyOptions)
}

func (options *settingsCommand) loadConfigAndInput() (browser.Config, browser.ApplyInput, error) {
	config, err := browser.LoadConfig(options.ConfigPath)
	if err != nil {
		return browser.Config{}, browser.ApplyInput{}, err
	}
	input, err := readApplyInput(options.InputPath)
	if err != nil {
		return browser.Config{}, browser.ApplyInput{}, err
	}
	return config, input, nil
}

func (*versionCommand) Run() error {
	fmt.Println(version)
	return nil
}

func readApplyInput(path string) (browser.ApplyInput, error) {
	if path == "" {
		return browser.ApplyInput{}, nil
	}
	var reader io.Reader = os.Stdin
	if path != standardInputPath {
		data, err := os.ReadFile(path)
		if err != nil {
			return browser.ApplyInput{}, fmt.Errorf("read input %s: %w", path, err)
		}
		reader = bytes.NewReader(data)
	}
	input, err := browser.DecodeApplyInput(reader)
	if err != nil {
		return browser.ApplyInput{}, fmt.Errorf("decode input %s: %w", path, err)
	}
	return input, nil
}
