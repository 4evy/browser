package chromium

import (
	"context"
	"os"
	"strings"

	foundation "github.com/4evy/tweezle/addons/foundation/chrome"
	"github.com/4evy/tweezle/addons/foundation/chrome/extensions"
)

type browserIdentity = foundation.BrowserIdentity

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
	identity, err := browser.readIdentity(ctx, launcher)
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
