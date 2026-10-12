// Package chromium provides Chromium profile storage and process primitives
// Browser identity, configured settings, and provider composition live above it
package chromium

import "context"

type BrowserIdentity = browserIdentity

type ExtensionStorageStage = extensionStorageStage

func ReadBrowserIdentity(ctx context.Context, launcher string) (BrowserIdentity, error) {
	return readBrowserIdentity(ctx, launcher)
}

func ReadBrowserIdentityCommand(ctx context.Context, prefix func(string) ([]string, error)) (BrowserIdentity, error) {
	return readBrowserIdentityCommand(ctx, prefix)
}

func StageExtensionStorage(profile, id string) (*ExtensionStorageStage, error) {
	return stageExtensionStorage(profile, id)
}

func (stage *extensionStorageStage) Root() string { return stage.root }

func (stage *extensionStorageStage) Remove() { stage.remove() }

func (stage *extensionStorageStage) Commit(profile string) error { return stage.commit(profile) }

func EvaluateExtensionCommand(ctx context.Context, prefix []string, directory, profile, pageURL, id, script string, input map[string]any) error {
	return evaluateExtensionCommand(ctx, prefix, directory, profile, pageURL, id, script, input)
}
