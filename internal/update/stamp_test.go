package update

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestStamp(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "ghx")
	if !IsDue(stateDir) {
		t.Fatal("expected due without a stamp")
	}
	if err := MarkChecked(stateDir); err != nil {
		t.Fatal(err)
	}
	if IsDue(stateDir) {
		t.Fatal("expected not due right after a check")
	}
	stale := time.Now().Add(-checkInterval - time.Minute)
	if err := os.Chtimes(filepath.Join(stateDir, stampName), stale, stale); err != nil {
		t.Fatal(err)
	}
	if !IsDue(stateDir) {
		t.Fatal("expected due after the interval")
	}
}

func TestStampAndLogArePrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	writers := map[string]func(stateDir string) error{
		stampName: MarkChecked,
		logName:   func(stateDir string) error { WriteLog(stateDir, "update failed"); return nil },
	}
	for name, write := range writers {
		t.Run(name+" new", func(t *testing.T) {
			stateDir := filepath.Join(t.TempDir(), "ghx")
			if err := write(stateDir); err != nil {
				t.Fatal(err)
			}
			assertMode(t, filepath.Join(stateDir, name), 0o600)
		})
		t.Run(name+" existing", func(t *testing.T) {
			stateDir := t.TempDir()
			path := filepath.Join(stateDir, name)
			if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := write(stateDir); err != nil {
				t.Fatal(err)
			}
			assertMode(t, path, 0o600)
		})
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s has mode %o, want %o", filepath.Base(path), got, want)
	}
}

func TestManagedGhPathEmptyWhenNotInstalled(t *testing.T) {
	if got := (ManagedGh{StateDir: t.TempDir()}).Path(); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestManagedGhInstallSetsCurrent(t *testing.T) {
	managed := ManagedGh{StateDir: t.TempDir()}
	if err := managed.install("2.0.0", []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := managed.install("2.1.0", []byte("new")); err != nil {
		t.Fatal(err)
	}
	if err := managed.install("2.2.0", []byte("newest")); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(managed.Path())
	if err != nil || string(content) != "newest" {
		t.Fatalf("got %q, %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(managed.root(), "2.0.0")); !os.IsNotExist(err) {
		t.Fatal("expected versions older than previous to be removed")
	}
	if _, err := os.Stat(filepath.Join(managed.root(), "2.1.0")); err != nil {
		t.Fatal("expected previous version to be kept")
	}
}
