package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/4evy/tweezle"
)

// ReportConfig owns [report], independently of the registered browser engine
type ReportConfig struct {
	Report struct {
		Message string `toml:"message"`
	} `toml:"report"`
}

func (config ReportConfig) Validate() error {
	if strings.TrimSpace(config.Report.Message) == "" {
		return errors.New("report.message is required")
	}
	return nil
}

// Report is an operation defined by this consumer
type Report struct{ Writer io.Writer }

func (config ReportConfig) Run(ctx context.Context, operation any) (bool, error) {
	options, ok := operation.(Report)
	if !ok {
		return false, nil
	}
	if err := ctx.Err(); err != nil {
		return true, err
	}
	if options.Writer == nil {
		return true, errors.New("report writer is required")
	}
	_, err := fmt.Fprintln(options.Writer, config.Report.Message)
	return true, err
}

var _ tweezle.Module = ReportConfig{}
