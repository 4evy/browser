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

	"github.com/4evy/browser/extensions"
	"github.com/4evy/browser/internal/jsonutil"
	"github.com/carlmjohnson/requests"
	"golang.org/x/net/http/httpguts"
)

const browserProbeTimeout = 15 * time.Second

type browserIdentity struct {
	UserAgent string `json:"User-Agent"`
	Product   string `json:"Browser"`
}

func (browser Browser) extensionHTTPClient(
	ctx context.Context,
	launcher string,
	excludedIDs map[string]bool,
) (extensions.HTTPClient, error) {
	client := browser.Extensions.Network.HTTPClient(os.Getenv(envGitHubToken))
	var chromeStore, downloads bool
	for _, entry := range browser.Extensions.ChromeStore {
		chromeStore = chromeStore || !excludedIDs[entry.ID]
	}
	for _, entry := range browser.Extensions.CRX {
		downloads = downloads || !excludedIDs[entry.ID]
	}
	for _, entry := range browser.Extensions.ZIP {
		downloads = downloads || !excludedIDs[entry.ID]
	}
	needsChromeVersion := chromeStore && client.ChromeVersion == ""
	needsUserAgent := (chromeStore || downloads) && client.UserAgent == ""
	if !needsChromeVersion && !needsUserAgent {
		return client, nil
	}
	identity, err := readBrowserIdentity(ctx, launcher)
	if err != nil {
		// When provisioning has no display or executable, use the latest Chrome
		// fallback while honoring caller cancellation
		return client, ctx.Err()
	}
	if client.UserAgent == "" {
		client.UserAgent = identity.UserAgent
	}
	if client.ChromeVersion == "" {
		product, version, _ := strings.Cut(identity.Product, "/")
		if (product == "Chrome" || product == "Chromium") &&
			extensions.ValidExternalVersion(version) {
			client.ChromeVersion = version
		}
	}
	return client, nil
}

// readBrowserIdentity asks the installed executable for its native user agent
// without touching the user's profile or synthesizing reduced/frozen tokens
// https://chromedevtools.github.io/devtools-protocol/#http-endpoints
func readBrowserIdentity(ctx context.Context, launcher string) (browserIdentity, error) {
	ctx, cancel := context.WithTimeout(ctx, browserProbeTimeout)
	defer cancel()
	profile, err := os.MkdirTemp("", "browser-user-agent-")
	if err != nil {
		return browserIdentity{}, err
	}
	defer removeBrowserProbeProfile(profile)
	// Headless mode changes the user agent, so use a windowless normal process
	command := exec.CommandContext(ctx, launcher,
		"--user-data-dir="+profile,
		"--remote-debugging-port=0",
		"--remote-debugging-address=127.0.0.1",
		"--no-first-run",
		"--no-default-browser-check",
		"--no-startup-window",
		"--disable-background-networking",
		"--disable-component-update",
	)
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
