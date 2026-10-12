package chromium

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func evaluateExtensionCommand(ctx context.Context, prefix []string, directory, profile, pageURL, id, script string, input map[string]any) error {
	var command *exec.Cmd
	allocatorOptions := []chromedp.ExecAllocatorOption{
		chromedp.ExecPath(prefix[0]),
		chromedp.UserDataDir(profile),
		chromedp.Flag("headless", "new"),
		chromedp.Flag("profile-directory", "Default"),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-component-update", true),
		chromedp.Flag("disable-sync", true),
		chromedp.ModifyCmdFunc(func(cmd *exec.Cmd) {
			cmd.Args = append(append([]string{prefix[0]}, prefix[1:]...), cmd.Args[1:]...)
			command = cmd
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
			cmd.WaitDelay = 2 * time.Second
		}),
	}
	if directory != "" {
		allocatorOptions = append(allocatorOptions, chromedp.Flag("load-extension", directory), chromedp.Flag("disable-extensions-except", directory))
	}
	allocatorCtx, stopAllocator := chromedp.NewExecAllocator(ctx, allocatorOptions...)
	browserCtx, stopBrowser := chromedp.NewContext(allocatorCtx)
	defer func() {
		stopBrowser()
		stopAllocator()
		// Children may outlive the browser and still own storage files
		if command != nil && command.Process != nil {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		}
	}()

	if err := chromedp.Do(browserCtx,
		chromedp.Navigate(pageURL),
		chromedp.PollFunction[chromedp.Void](
			`(url, id) => location.href === new URL(url).href && globalThis.chrome?.runtime?.id === id`,
			chromedp.WithPollingArgs(pageURL, id),
			chromedp.WithPollingTimeout(0),
		),
	); err != nil {
		return fmt.Errorf("open extension page: %w", err)
	}
	global, err := chromedp.Run(browserCtx, chromedp.Evaluate[*runtime.RemoteObject]("globalThis"))
	if err != nil {
		return fmt.Errorf("find extension page context: %w", err)
	}
	await := true
	if _, err := chromedp.Run(browserCtx, chromedp.CallFunctionOn[chromedp.Void](
		"async function(input) {\n"+script+"\n}",
		func(params *runtime.CallFunctionOnParams) {
			params.ObjectID = global.ObjectID
			params.AwaitPromise = &await
		},
		input,
	)); err != nil {
		return fmt.Errorf("extension script failed: %w", err)
	}
	// A graceful close flushes databases; cancellation alone kills the browser
	if err := chromedp.Cancel(browserCtx); err != nil {
		return fmt.Errorf("close temporary browser: %w", err)
	}
	stopAllocator()
	if err := ctx.Err(); err != nil {
		return err
	}
	if command.ProcessState == nil || !command.ProcessState.Success() {
		return fmt.Errorf("temporary browser did not exit cleanly: %v", command.ProcessState)
	}
	return nil
}
