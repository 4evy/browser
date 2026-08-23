package extensions

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitCloneURLSupportsHostedProvidersAndGenericGit(t *testing.T) {
	tests := []struct {
		name      string
		extension GitExtension
		want      string
	}{
		{
			name: "GitHub", extension: GitExtension{
				Provider: GitProviderGitHub, Repository: "owner/project",
			},
			want: "https://github.com/owner/project.git",
		},
		{
			name: "nested GitLab namespace", extension: GitExtension{
				Provider: GitProviderGitLab, Repository: "group/team/project",
			},
			want: "https://gitlab.com/group/team/project.git",
		},
		{
			name: "Codeberg", extension: GitExtension{
				Provider: GitProviderCodeberg, Repository: "owner/project",
			},
			want: "https://codeberg.org/owner/project.git",
		},
		{
			name: "SourceHut", extension: GitExtension{
				Provider: GitProviderSourceHut, Repository: "~owner/project",
			},
			want: "https://git.sr.ht/~owner/project",
		},
		{
			name: "generic URL with inferred provider", extension: GitExtension{
				URL: "ssh://git@example.test/team/project.git",
			},
			want: "ssh://git@example.test/team/project.git",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := GitCloneURL(test.extension)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("clone URL = %q, want %q", got, test.want)
			}
		})
	}
}

func TestGitClientExportsPinnedSubdirectoryWithoutRepositoryMetadata(t *testing.T) {
	gitPath, err := exec.LookPath(defaultGitExecutable)
	if err != nil {
		t.Skip("git is not installed")
	}
	repository := t.TempDir()
	runTestGit(t, gitPath, repository, "init", "--quiet")
	runTestGit(t, gitPath, repository, "config", "user.name", "Test")
	runTestGit(t, gitPath, repository, "config", "user.email", "test@example.test")
	extensionDir := filepath.Join(repository, "dist", "extension")
	if err := os.MkdirAll(extensionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(extensionDir, manifestFilename),
		[]byte(`{"name":"Git extension","version":"1.2.3"}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extensionDir, "worker.js"), []byte("export {};"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, gitPath, repository, "add", ".")
	runTestGit(t, gitPath, repository, "commit", "--quiet", "-m", "extension")
	commit := runTestGit(t, gitPath, repository, "rev-parse", "HEAD")
	ref := runTestGit(t, gitPath, repository, "symbolic-ref", "--short", "HEAD")
	repositoryURL := (&url.URL{Scheme: "file", Path: repository}).String()
	destination := t.TempDir()

	revision, err := (GitClient{Executable: gitPath}).Checkout(t.Context(), destination, GitExtension{
		Provider:     GitProviderGit,
		URL:          repositoryURL,
		UpdatePolicy: UpdatePolicyPinned,
		Ref:          ref,
		Commit:       commit,
		Subdirectory: "dist/extension",
	})
	if err != nil {
		t.Fatal(err)
	}
	if revision.Commit != commit {
		t.Fatalf("revision commit = %q, want %q", revision.Commit, commit)
	}
	for _, path := range []string{
		filepath.Join(destination, manifestFilename),
		filepath.Join(destination, "worker.js"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(destination, ".git")); !os.IsNotExist(err) {
		t.Fatalf("exported extension contains .git metadata: %v", err)
	}

	latestDestination := t.TempDir()
	latestRevision, err := (GitClient{Executable: gitPath}).Checkout(
		t.Context(),
		latestDestination,
		GitExtension{
			URL:          repositoryURL,
			UpdatePolicy: UpdatePolicyLatest,
			Subdirectory: "dist/extension",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if latestRevision.Commit != commit {
		t.Fatalf("latest revision commit = %q, want default-branch %q", latestRevision.Commit, commit)
	}
}

func TestGitClientRejectsRefThatDoesNotMatchPinnedCommit(t *testing.T) {
	gitPath, err := exec.LookPath(defaultGitExecutable)
	if err != nil {
		t.Skip("git is not installed")
	}
	repository := t.TempDir()
	runTestGit(t, gitPath, repository, "init", "--quiet")
	runTestGit(t, gitPath, repository, "config", "user.name", "Test")
	runTestGit(t, gitPath, repository, "config", "user.email", "test@example.test")
	if err := os.WriteFile(filepath.Join(repository, "manifest.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, gitPath, repository, "add", ".")
	runTestGit(t, gitPath, repository, "commit", "--quiet", "-m", "extension")
	ref := runTestGit(t, gitPath, repository, "symbolic-ref", "--short", "HEAD")
	repositoryURL := (&url.URL{Scheme: "file", Path: repository}).String()

	_, err = (GitClient{Executable: gitPath}).Checkout(t.Context(), t.TempDir(), GitExtension{
		Provider:     GitProviderGit,
		URL:          repositoryURL,
		UpdatePolicy: UpdatePolicyPinned,
		Ref:          ref,
		Commit:       strings.Repeat("0", 40),
	})
	if err == nil || !strings.Contains(err.Error(), "want pinned commit") {
		t.Fatalf("checkout error = %v, want pin mismatch", err)
	}
}

func runTestGit(t *testing.T, executable, directory string, arguments ...string) string {
	t.Helper()
	command := exec.CommandContext(context.Background(), executable, arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", arguments[0], err, output)
	}
	return strings.TrimSpace(string(output))
}
