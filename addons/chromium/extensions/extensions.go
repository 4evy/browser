// Package extensions exposes the shared implementation in addons/foundation
package extensions

import (
	"context"

	foundation "github.com/4evy/tweezle/addons/foundation/chrome/extensions"
)

type UpdatePolicy = foundation.UpdatePolicy

type GitProvider = foundation.GitProvider

const UpdatePolicyLatest = foundation.UpdatePolicyLatest

const UpdatePolicyPinned = foundation.UpdatePolicyPinned

const GitProviderGit = foundation.GitProviderGit

const GitProviderGitHub = foundation.GitProviderGitHub

const GitProviderGitLab = foundation.GitProviderGitLab

const GitProviderCodeberg = foundation.GitProviderCodeberg

const GitProviderSourceHut = foundation.GitProviderSourceHut

type Catalog = foundation.Catalog

type ExtensionIdentity = foundation.ExtensionIdentity

type NetworkConfig = foundation.NetworkConfig

type ChromeStoreExtension = foundation.ChromeStoreExtension

type UpdateURLExtension = foundation.UpdateURLExtension

type DownloadedExtension = foundation.DownloadedExtension

type ZIPExtension = foundation.ZIPExtension

type GitHubReleaseExtension = foundation.GitHubReleaseExtension

type GitExtension = foundation.GitExtension

type ReleaseArtifact = foundation.ReleaseArtifact

type DownloadFunc = foundation.DownloadFunc

type ResolveURLFunc = foundation.ResolveURLFunc

type ResolveLatestReleaseFunc = foundation.ResolveLatestReleaseFunc

type GitRevision = foundation.GitRevision

type CheckoutGitFunc = foundation.CheckoutGitFunc

type Options = foundation.Options

type Result = foundation.Result

type GitClient = foundation.GitClient

func GitCloneURL(extension GitExtension) (string, error) {
	return foundation.GitCloneURL(extension)
}

func Install(ctx context.Context, options Options) (Result, error) {
	return foundation.Install(ctx, options)
}

func UnpackedManifestID(directory string) (string, error) {
	return foundation.UnpackedManifestID(directory)
}

func ExtractZipFile(zipPath, destination string) (err error) {
	return foundation.ExtractZipFile(zipPath, destination)
}

func VerifyFileSHA256(path, checksum string) (err error) {
	return foundation.VerifyFileSHA256(path, checksum)
}

func UnpackedExtensionID(extensionPath string) string {
	return foundation.UnpackedExtensionID(extensionPath)
}

func ChromeStoreCRXDownloadURLForVersion(updateURL, id, chromeVersion string) (string, error) {
	return foundation.ChromeStoreCRXDownloadURLForVersion(updateURL, id, chromeVersion)
}

func ChromeStoreVersionFromCRXURL(id, crxURL string) (string, error) {
	return foundation.ChromeStoreVersionFromCRXURL(id, crxURL)
}

type HTTPClient = foundation.HTTPClient

func ValidateCatalog(catalog Catalog) error {
	return foundation.ValidateCatalog(catalog)
}

func ValidateIDAliases(aliases map[string]string) error {
	return foundation.ValidateIDAliases(aliases)
}

func ValidExtensionID(id string) bool {
	return foundation.ValidExtensionID(id)
}

func ValidURL(rawURL string) bool {
	return foundation.ValidURL(rawURL)
}

func ValidExternalVersion(version string) bool {
	return foundation.ValidExternalVersion(version)
}

func ValidGitHubRepository(repository string) bool {
	return foundation.ValidGitHubRepository(repository)
}

func ValidHostedGitRepository(provider GitProvider, repository string) bool {
	return foundation.ValidHostedGitRepository(provider, repository)
}

func ValidGitURL(rawURL string) bool {
	return foundation.ValidGitURL(rawURL)
}

func ValidGitRef(ref string) bool {
	return foundation.ValidGitRef(ref)
}

func ValidGitCommit(commit string) bool {
	return foundation.ValidGitCommit(commit)
}

func ValidSHA256(checksum string) bool {
	return foundation.ValidSHA256(checksum)
}

func NormalizeSHA256(checksum string) (string, error) {
	return foundation.NormalizeSHA256(checksum)
}
