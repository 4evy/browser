package chromium

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	jsonutil "github.com/4evy/tweezle/internal/json"
	"github.com/carlmjohnson/requests"
	"golang.org/x/net/http/httpguts"
)

const browserProbeTimeout = 15 * time.Second

type browserIdentity struct {
	UserAgent string `json:"User-Agent"`
	Product   string `json:"Browser"`
}

// readBrowserIdentity asks the installed executable for its native user agent
// without touching the user's profile or synthesizing reduced/frozen tokens
// https://chromedevtools.github.io/devtools-protocol/#http-endpoints
func readBrowserIdentity(ctx context.Context, launcher string) (browserIdentity, error) {
	return readBrowserIdentityCommand(ctx, func(profile string) ([]string, error) {
		return []string{launcher}, nil
	})
}

func readBrowserIdentityCommand(ctx context.Context, prefix func(string) ([]string, error)) (browserIdentity, error) {
	ctx, cancel := context.WithTimeout(ctx, browserProbeTimeout)
	defer cancel()
	profile, err := os.MkdirTemp("", "tweezle-user-agent-")
	if err != nil {
		return browserIdentity{}, err
	}
	defer removeBrowserProbeProfile(profile)
	commandPrefix, err := prefix(profile)
	if err != nil {
		return browserIdentity{}, err
	}
	// Headless mode changes the user agent, so use a windowless normal process
	args := append(commandPrefix[1:],
		"--user-data-dir="+profile,
		"--remote-debugging-port=0",
		"--remote-debugging-address=127.0.0.1",
		"--no-first-run",
		"--no-default-browser-check",
		"--no-startup-window",
		"--disable-background-networking",
		"--disable-component-update",
	)
	command := exec.CommandContext(ctx, commandPrefix[0], args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGTERM) }
	command.WaitDelay = time.Second
	if err := command.Start(); err != nil {
		return browserIdentity{}, err
	}
	done := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = command.Wait()
		close(done)
	}()
	defer func() {
		cancel()
		<-done
		// Browser children can outlive the main process and still write
		// profiles
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, err := os.ReadFile(filepath.Join(profile, "DevToolsActivePort"))
		if err == nil {
			line, _, _ := strings.Cut(string(data), "\n")
			port, err := strconv.Atoi(line)
			if err == nil && port > 0 && port <= 65535 {
				return readDevToolsIdentity(ctx, port)
			}
		}
		select {
		case <-ctx.Done():
			return browserIdentity{}, ctx.Err()
		case <-done:
			return browserIdentity{}, fmt.Errorf("browser exited before exposing DevTools: %v", waitErr)
		case <-ticker.C:
		}
	}
}

func removeBrowserProbeProfile(profile string) {
	// Allow killed children to release the profile before retrying removal
	for range 10 {
		if err := os.RemoveAll(profile); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func readDevToolsIdentity(ctx context.Context, port int) (browserIdentity, error) {
	// Local DevTools requests must bypass configured HTTP proxies
	client := &http.Client{Transport: &http.Transport{}}
	defer client.CloseIdleConnections()
	var identity browserIdentity
	err := requests.URL(fmt.Sprintf("http://127.0.0.1:%d/json/version", port)).
		Client(client).
		CheckStatus(http.StatusOK).
		Handle(func(response *http.Response) error {
			return jsonutil.Default.DecodeInto(io.LimitReader(response.Body, 64*1024), &identity)
		}).
		Fetch(ctx)
	if err != nil {
		return browserIdentity{}, err
	}
	if identity.UserAgent == "" || !httpguts.ValidHeaderFieldValue(identity.UserAgent) {
		return browserIdentity{}, fmt.Errorf("DevTools returned an invalid user agent")
	}
	return identity, nil
}
