// This example builds a consumer-owned CLI using only public Tweezle packages
// Run from the repository root with:
//
//	go run ./examples/consumer config validate examples/consumer/consumer.toml
//	go run ./examples/consumer report examples/consumer/consumer.toml
//
// CustomProvider extends Chromium alongside bundled Brave and Helium providers
// ReportConfig adds a separate TOML table and a consumer-defined operation
// The reusable CLI supplies apply, profile, storage, policy, and config commands
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/4evy/tweezle"
	"github.com/4evy/tweezle/addons/brave"
	"github.com/4evy/tweezle/addons/chromium"
	"github.com/4evy/tweezle/addons/helium"
	"github.com/4evy/tweezle/cli"
	"github.com/4evy/tweezle/launcher"
	"github.com/spf13/cobra"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	if handled, err := launcher.Run(os.Args[0], os.Args[1:]); handled {
		return err
	}
	addons := []tweezle.Addon{
		chromium.Addon(
			helium.Addon(),
			brave.Addon(),
			chromium.NewProviderAddon[CustomProvider]("custom"),
		),
		tweezle.NewAddon[ReportConfig]("report", "report"),
	}
	command, err := cli.NewCommand(cli.Options{
		Addons:  addons,
		Version: "consumer-example",
	})
	if err != nil {
		return err
	}
	command.Use = "consumer"
	// This application prints returned errors itself
	command.SilenceErrors = true
	command.AddGroup(&cobra.Group{ID: "consumer", Title: "Consumer Commands:"})
	command.AddCommand(&cobra.Command{
		Use:     "report CONFIG",
		Short:   "Print the consumer's configured message",
		GroupID: "consumer",
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			config, err := tweezle.LoadConfig(arguments[0], addons...)
			if err != nil {
				return err
			}
			return config.Run(command.Context(), Report{Writer: command.OutOrStdout()})
		},
	})
	command.SetArgs(os.Args[1:])
	return command.ExecuteContext(ctx)
}
