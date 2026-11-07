package update

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/barnuri/ghx/internal/release"
)

const (
	ghxRepo          = "barnuri/ghx"
	ghxChecksumsName = "checksums.txt"
	devVersion       = "dev"
)

// ReleaseClient is the part of release.Client the self-updater uses, so tests can fake it.
type ReleaseClient interface {
	Latest(repo string) (release.Release, error)
	DownloadVerified(asset release.Asset, checksums release.Asset) ([]byte, error)
}

type SelfUpdater struct {
	Client         ReleaseClient
	CurrentVersion string
}

// Update replaces the running ghx binary with the latest release when that release is newer.
// It returns the new version, or "" when ghx is current, ahead of the latest release, or a
// local dev build. An unparsable version on either side is an error, never an update.
func (updater SelfUpdater) Update() (string, error) {
	if updater.CurrentVersion == devVersion {
		return "", nil
	}
	latest, err := updater.Client.Latest(ghxRepo)
	if err != nil {
		return "", err
	}
	order, err := compareVersions(latest.Version(), updater.CurrentVersion)
	if err != nil {
		return "", fmt.Errorf("compare release %s with ghx %s: %w", latest.TagName, updater.CurrentVersion, err)
	}
	if order <= 0 {
		return "", nil
	}
	asset, found := latest.Find(ghxAssetName())
	checksums, hasChecksums := latest.Find(ghxChecksumsName)
	if !found || !hasChecksums {
		return "", fmt.Errorf("release %s has no %s or %s", latest.TagName, ghxAssetName(), ghxChecksumsName)
	}
	content, err := updater.Client.DownloadVerified(asset, checksums)
	if err != nil {
		return "", err
	}
	if err := replaceExecutable(content); err != nil {
		return "", err
	}
	return latest.Version(), nil
}

// replaceExecutable swaps the binary by rename. On Windows the gh.exe shim is a copy of
// ghx.exe in the same dir, so both copies are replaced. A running .exe can be renamed, not
// overwritten, so each old file moves aside first.
func replaceExecutable(content []byte) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	if runtime.GOOS != "windows" {
		return replaceFile(executable, content)
	}
	installDir := filepath.Dir(executable)
	for _, name := range []string{"ghx.exe", "gh.exe"} {
		target := filepath.Join(installDir, name)
		if _, err := os.Stat(target); err != nil && target != executable {
			continue
		}
		if err := replaceFile(target, content); err != nil {
			return err
		}
	}
	return nil
}

func replaceFile(target string, content []byte) error {
	staged := target + ".new"
	if err := os.WriteFile(staged, content, 0o755); err != nil {
		return fmt.Errorf("write %s: %w", staged, err)
	}
	if runtime.GOOS == "windows" {
		previous := target + ".old"
		_ = os.Remove(previous)
		if err := os.Rename(target, previous); err != nil {
			return err
		}
	}
	return os.Rename(staged, target)
}
