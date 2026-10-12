package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/4evy/tweezle/cli"
)

var version = "dev"

func main() {
	os.Exit(mainResult())
}

func mainResult() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cli.Run(ctx, os.Args[1:], cli.Options{
		Addons:  bundledAddons(),
		Version: version,
	}); err != nil {
		if errors.Is(err, context.Canceled) {
			return 130
		}
		return 1
	}
	return 0
}
