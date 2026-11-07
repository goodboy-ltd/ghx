package ghbin

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLocateFindsGhOnPath(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	expected := filepath.Join(second, BinaryName())
	if err := os.WriteFile(expected, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ghPathEnv, "")
	t.Setenv("PATH", first+string(os.PathListSeparator)+second)
	got, err := Locate()
	if err != nil || got != expected {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestLocateSkipsSelf(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	if err := os.Symlink(executable, filepath.Join(shimDir, BinaryName())); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	t.Setenv(ghPathEnv, "")
	t.Setenv("PATH", shimDir)
	withFallbackDirs(t, nil)
	if _, err := Locate(); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestLocateEnvOverride(t *testing.T) {
	absolute := filepath.Join(t.TempDir(), BinaryName())
	t.Setenv(ghPathEnv, absolute)
	if got, err := Locate(); err != nil || got != absolute {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestLocateRejectsRelativeEnvOverride(t *testing.T) {
	for _, value := range []string{"gh", "./gh", "bin/gh", "../gh"} {
		t.Run(value, func(t *testing.T) {
			t.Chdir(dirWithGh(t))
			t.Setenv(ghPathEnv, value)
			got, err := Locate()
			if !errors.Is(err, ErrRelativeOverride) || got != "" {
				t.Fatalf("got %q, %v; want ErrRelativeOverride", got, err)
			}
		})
	}
}

// TestLocateSkipsRelativePathEntries checks that an empty or relative PATH entry never yields a
// gh that sits in the current directory, which would run with the profile's token.
func TestLocateSkipsRelativePathEntries(t *testing.T) {
	sep := string(os.PathListSeparator)
	cases := []struct {
		name string
		path func(empty, real string) string
		real bool
	}{
		{"leading separator", func(empty, _ string) string { return sep + empty }, false},
		{"trailing separator", func(empty, _ string) string { return empty + sep }, false},
		{"double separator", func(empty, _ string) string { return empty + sep + sep + empty }, false},
		{"only a separator", func(_, _ string) string { return sep }, false},
		{"dot entry", func(empty, _ string) string { return "." + sep + empty }, false},
		{"relative entry", func(empty, _ string) string { return "sub" + sep + empty }, false},
		{"empty entry before a real gh", func(_, real string) string { return sep + real }, true},
		{"double separator before a real gh", func(empty, real string) string { return empty + sep + sep + real }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cwd := dirWithGh(t)
			if err := os.Mkdir(filepath.Join(cwd, "sub"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(cwd, "sub", BinaryName()), []byte("x"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Chdir(cwd)
			real := dirWithGh(t)
			t.Setenv(ghPathEnv, "")
			t.Setenv("PATH", tc.path(t.TempDir(), real))
			withFallbackDirs(t, nil)
			got, err := Locate()
			if tc.real {
				want := filepath.Join(real, BinaryName())
				if err != nil || got != want {
					t.Fatalf("got %q, %v; want %q", got, err, want)
				}
				return
			}
			if err != ErrNotFound {
				t.Fatalf("got %q, %v; want ErrNotFound", got, err)
			}
		})
	}
}

func TestLocateFallsBackWhenPathLacksGh(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no fallback dirs on Windows")
	}
	fallback := t.TempDir()
	expected := filepath.Join(fallback, BinaryName())
	if err := os.WriteFile(expected, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ghPathEnv, "")
	t.Setenv("PATH", t.TempDir())
	withFallbackDirs(t, []string{fallback})
	got, err := Locate()
	if err != nil || got != expected {
		t.Fatalf("got %q, %v", got, err)
	}
}

func withFallbackDirs(t *testing.T, dirs []string) {
	saved := fallbackDirs
	fallbackDirs = dirs
	t.Cleanup(func() { fallbackDirs = saved })
}

// dirWithGh returns a new temp dir holding an executable gh.
func dirWithGh(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, BinaryName()), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}
