package extensions

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	defaultGitExecutable = "git"
	gitTemporaryPrefix   = "browser-extension-git-"
)

type GitClient struct {
	Executable string
}

func GitCloneURL(extension GitExtension) (string, error) {
	provider := extension.gitProvider()
	if provider == GitProviderGit {
		if extension.Repository != "" || !ValidGitURL(extension.URL) {
			return "", fmt.Errorf("invalid Git URL %q", extension.URL)
		}
		return extension.URL, nil
	}
	if extension.URL != "" {
		return "", fmt.Errorf("%s Git source must not set URL", provider)
	}
	if !ValidHostedGitRepository(provider, extension.Repository) {
		return "", fmt.Errorf(
			"invalid %s repository %q",
			provider,
			extension.Repository,
		)
	}
	switch provider {
	case GitProviderGitHub:
		return "https://github.com/" + extension.Repository + ".git", nil
	case GitProviderGitLab:
		return "https://gitlab.com/" + extension.Repository + ".git", nil
	case GitProviderCodeberg:
		return "https://codeberg.org/" + extension.Repository + ".git", nil
	case GitProviderSourceHut:
		return "https://git.sr.ht/" + extension.Repository, nil
	default:
		return "", fmt.Errorf("unsupported Git provider %q", provider)
	}
}

func (client GitClient) Checkout(
	ctx context.Context,
	destination string,
	extension GitExtension,
) (_ GitRevision, err error) {
	cloneURL, err := GitCloneURL(extension)
	if err != nil {
		return GitRevision{}, err
	}
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return GitRevision{}, fmt.Errorf("create Git export destination: %w", err)
	}
	entries, err := os.ReadDir(destination)
	if err != nil {
		return GitRevision{}, fmt.Errorf("read Git export destination: %w", err)
	}
	if len(entries) != 0 {
		return GitRevision{}, errors.New("git export destination must be empty")
	}
	workDir, err := os.MkdirTemp("", gitTemporaryPrefix)
	if err != nil {
		return GitRevision{}, err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(workDir)) }()

	repositoryDir := filepath.Join(workDir, "repository")
	archivePath := filepath.Join(workDir, "extension.zip")
	if err := client.run(ctx, "initialize repository", "", cloneURL,
		"init", "--quiet", repositoryDir,
	); err != nil {
		return GitRevision{}, err
	}
	if err := client.run(ctx, "configure repository remote", "", cloneURL,
		"-C", repositoryDir, "remote", "add", "origin", cloneURL,
	); err != nil {
		return GitRevision{}, err
	}
	fetchTarget := extension.Ref
	if extension.updatePolicy() == UpdatePolicyPinned && fetchTarget == "" {
		fetchTarget = extension.Commit
	}
	if fetchTarget == "" {
		fetchTarget = "HEAD"
	}
	if err := client.run(ctx, "fetch repository revision", "", cloneURL,
		"-c", "core.hooksPath=/dev/null",
		"-C", repositoryDir,
		"fetch", "--quiet", "--depth=1", "origin", fetchTarget,
	); err != nil {
		return GitRevision{}, err
	}
	output, err := client.output(ctx, "resolve repository revision", repositoryDir, cloneURL,
		"rev-parse", "--verify", "FETCH_HEAD^{commit}",
	)
	if err != nil {
		return GitRevision{}, err
	}
	commit := strings.TrimSpace(output)
	if !ValidGitCommit(commit) {
		return GitRevision{}, fmt.Errorf("git returned invalid commit %q", commit)
	}
	if extension.updatePolicy() == UpdatePolicyPinned &&
		!strings.EqualFold(commit, extension.Commit) {
		return GitRevision{}, fmt.Errorf(
			"git ref %q resolved to commit %s, want pinned commit %s",
			extension.Ref,
			commit,
			extension.Commit,
		)
	}
	treeish := commit
	if extension.Subdirectory != "" {
		treeish += ":" + extension.Subdirectory
	}
	if err := client.run(ctx, "export repository revision", repositoryDir, cloneURL,
		"archive", "--format=zip", "--output", archivePath, treeish,
	); err != nil {
		return GitRevision{}, err
	}
	if err := ExtractZipFile(archivePath, destination); err != nil {
		return GitRevision{}, fmt.Errorf("extract Git revision %s: %w", commit, err)
	}
	return GitRevision{Commit: commit}, nil
}

func (client GitClient) run(
	ctx context.Context,
	operation,
	directory,
	cloneURL string,
	arguments ...string,
) error {
	_, err := client.output(ctx, operation, directory, cloneURL, arguments...)
	return err
}

func (client GitClient) output(
	ctx context.Context,
	operation,
	directory,
	cloneURL string,
	arguments ...string,
) (string, error) {
	executable := client.Executable
	if executable == "" {
		executable = defaultGitExecutable
	}
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := command.CombinedOutput()
	if err == nil {
		return string(output), nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", fmt.Errorf("%s: %w", operation, ctxErr)
	}
	detail := strings.TrimSpace(string(output))
	if detail != "" {
		detail = strings.ReplaceAll(detail, cloneURL, safeGitURL(cloneURL))
		return "", fmt.Errorf("%s: %w: %s", operation, err, detail)
	}
	return "", fmt.Errorf("%s: %w", operation, err)
}

func safeGitURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.User == nil {
		return rawURL
	}
	parsed.User = nil
	return parsed.String()
}
