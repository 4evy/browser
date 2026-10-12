package firefox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileLockContention(t *testing.T) {
	if directory := os.Getenv("TWEEZLE_TEST_LOCK_PROFILE"); directory != "" {
		unlock, err := lockProfile(directory)
		if err == nil {
			unlock()
			t.Fatal("child acquired parent's profile lock")
		}
		return
	}
	directory := t.TempDir()
	unlock, err := lockProfile(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	alias := filepath.Join(t.TempDir(), "profile")
	if err := os.Symlink(directory, alias); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{directory, alias} {
		second, err := lockProfile(path)
		if err == nil {
			second()
			t.Fatalf("same process acquired active lock through %s", path)
		}
	}
	t.Run("case alias", func(t *testing.T) {
		caseAlias := strings.ToUpper(directory)
		original, err := os.Stat(directory)
		if err != nil {
			t.Fatal(err)
		}
		aliased, err := os.Stat(caseAlias)
		if err != nil || !os.SameFile(original, aliased) {
			t.Skip("filesystem does not provide a case alias")
		}
		second, err := lockProfile(caseAlias)
		if err == nil {
			second()
			t.Fatal("acquired active profile through a case alias")
		}
	})
	// Failed attempts in this process must not release the kernel lock
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestProfileLockContention$")
	command.Env = append(os.Environ(), "TWEEZLE_TEST_LOCK_PROFILE="+directory)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("subprocess: %v\n%s", err, output)
	}
	unlock()
	unlock()
	again, err := lockProfile(directory)
	if err != nil {
		t.Fatal(err)
	}
	again()
}

func TestProfileLockFailureReleasesReservation(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "lock")
	if err := os.Symlink("unknown-host:1234", path); err != nil {
		t.Fatal(err)
	}
	if unlock, err := lockProfile(directory); err == nil {
		unlock()
		t.Fatal("accepted unknown symlink lock")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	unlock, err := lockProfile(directory)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}
