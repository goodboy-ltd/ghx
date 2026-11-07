package ghbin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const ghPathEnv = "GHX_GH"

var ErrNotFound = errors.New("gh not found on PATH; install gh or set manage_gh: true in the ghx config")

// ErrRelativeOverride is returned when GHX_GH is not an absolute path. A relative one would
// resolve against the current directory, so a repository holding a gh of its own would run with
// the profile's token.
var ErrRelativeOverride = errors.New(ghPathEnv + " must be an absolute path")

// fallbackDirs are the usual gh install dirs. Apps started from the macOS Dock (VS Code, GUI
// git clients) get a PATH without them, so git credential calls would not find gh.
var fallbackDirs = []string{"/opt/homebrew/bin", "/usr/local/bin", "/home/linuxbrew/.linuxbrew/bin"}

func BinaryName() string {
	if runtime.GOOS == "windows" {
		return "gh.exe"
	}
	return "gh"
}

// Locate finds the real gh. It skips ghx itself and the ghx install dir, so a gh shim that
// points at ghx never resolves to itself. It never returns a relative path: an empty or relative
// PATH entry is skipped, as exec.LookPath has refused one since Go 1.19, and GHX_GH must be
// absolute.
func Locate() (string, error) {
	if fromEnv := os.Getenv(ghPathEnv); fromEnv != "" {
		if !filepath.IsAbs(fromEnv) {
			return "", fmt.Errorf("%w, got %q", ErrRelativeOverride, fromEnv)
		}
		return fromEnv, nil
	}
	dirs := filepath.SplitList(os.Getenv("PATH"))
	if runtime.GOOS != "windows" {
		dirs = append(dirs, fallbackDirs...)
	}
	selfPath, selfInfo := self()
	for _, dir := range dirs {
		// An empty entry (a leading, trailing or doubled separator) and a relative one both
		// name a directory relative to the current one.
		if !filepath.IsAbs(dir) {
			continue
		}
		candidate := filepath.Join(dir, BinaryName())
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if selfInfo != nil && (os.SameFile(selfInfo, info) || sameDir(selfPath, candidate)) {
			continue
		}
		return candidate, nil
	}
	return "", ErrNotFound
}

func self() (string, os.FileInfo) {
	executable, err := os.Executable()
	if err != nil {
		return "", nil
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	info, err := os.Stat(executable)
	if err != nil {
		return "", nil
	}
	return executable, info
}

func sameDir(selfPath string, candidate string) bool {
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return false
	}
	return filepath.Dir(resolved) == filepath.Dir(selfPath)
}
