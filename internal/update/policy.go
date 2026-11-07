package update

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	PackageManagerHomebrew = "homebrew"
	PackageManagerAsdf     = "asdf"
)

// ErrSelfUpdateDisabled is returned by SelfUpdater.Update when self-update is off.
var ErrSelfUpdateDisabled = errors.New("ghx self-update is off")

// Policy says where ghx updates itself from, whether it may, and whether a package manager
// owns the installed binary instead.
type Policy struct {
	Repo    string
	Enabled bool
	// Reason names what set Enabled: "build default", "self_update: true" or "self_update: false".
	Reason    string
	ManagedBy string
}

// Active reports whether ghx may replace its own binary.
func (policy Policy) Active() bool {
	return policy.Enabled && policy.ManagedBy == ""
}

// Describe is the value of the "updates:" line in gh ghx which.
func (policy Policy) Describe() string {
	switch {
	case policy.ManagedBy != "":
		return "managed by " + policy.ManagedBy
	case !policy.Enabled:
		return "off (" + policy.Reason + ")"
	}
	return "on from " + policy.Repo
}

func (policy Policy) refusal() error {
	if policy.ManagedBy != "" {
		return fmt.Errorf("ghx is installed by %s; run: %s", policy.ManagedBy, UpgradeCommand(policy.ManagedBy))
	}
	if !policy.Enabled {
		return fmt.Errorf("%w (%s)", ErrSelfUpdateDisabled, policy.Reason)
	}
	if policy.Repo == "" {
		return errors.New("no update repository configured")
	}
	return nil
}

// UpgradeCommand is the command that updates ghx through packageManager.
func UpgradeCommand(packageManager string) string {
	if packageManager == PackageManagerAsdf {
		return "asdf install ghx latest"
	}
	return "brew upgrade ghx"
}

// DetectPackageManager returns the package manager whose install directory holds executable,
// a symlink-resolved path: a Homebrew Cellar (.../Cellar/ghx/<version>/bin/ghx) or an asdf
// installs directory (.../installs/ghx/<version>/bin/ghx). It returns "" for anything else.
func DetectPackageManager(executable string) string {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(executable)), "/")
	if len(parts) < 5 || parts[len(parts)-2] != "bin" || parts[len(parts)-4] != "ghx" {
		return ""
	}
	switch parts[len(parts)-5] {
	case "Cellar":
		return PackageManagerHomebrew
	case "installs":
		return PackageManagerAsdf
	}
	return ""
}
