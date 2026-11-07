package resolve

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/barnuri/ghx/internal/config"
)

const (
	RepoFileName     = ".ghx"
	overrideSource   = "GHX_PROFILE"
	defaultSource    = "default"
	sshHostSource    = "ssh host "
	remoteUserSource = "remote user "
)

// Profile picks the profile for cwd in this order: GHX_PROFILE, the nearest .ghx file,
// the ssh host or https user of the origin remote, the deepest configured dir containing cwd (or, in a
// linked git worktree, its main checkout), then default. No match means plain gh. The error is an
// *UnknownProfileError, returned only with profiles_only.
func Profile(cfg config.Config, cwd string, override string, remoteURL RemoteURL) (Match, error) {
	if override != "" {
		return requested(cfg, override, overrideSource)
	}
	current := canonical(cwd)
	found := nearestRepoFile(current, cfg.StrictRepoFile())
	match, err := fromRules(cfg, current, found, remoteURL)
	match.Ignored = found.ignored
	return match, err
}

func fromRules(cfg config.Config, current string, found repoFile, remoteURL RemoteURL) (Match, error) {
	if found.name != "" {
		return requested(cfg, found.name, found.path)
	}
	if match := fromOrigin(cfg, current, remoteURL); match.Found() {
		return match, nil
	}
	if match := dirsMatch(cfg, current); match.Found() {
		return match, nil
	}
	if cfg.Default != "" {
		return named(cfg, cfg.Default, defaultSource), nil
	}
	return Match{}, nil
}

// UnknownProfileError reports a GHX_PROFILE value or a .ghx file that names no profile
// while profiles_only is set. Source is GHX_PROFILE or the path of the .ghx file.
type UnknownProfileError struct {
	Name   string
	Source string
}

func (err *UnknownProfileError) Error() string {
	return fmt.Sprintf("%s names %q, which is not a profile (profiles_only: true)", err.Source, err.Name)
}

// requested resolves a name someone asked for explicitly, in GHX_PROFILE or a .ghx file.
// With profiles_only, a name that is not a profile is an error: falling back to another
// rule would run gh as an account other than the one asked for.
func requested(cfg config.Config, name string, source string) (Match, error) {
	if _, ok := cfg.Profiles[name]; !ok && cfg.ProfilesOnly {
		return Match{}, &UnknownProfileError{Name: name, Source: source}
	}
	return named(cfg, name, source), nil
}

// named treats a name with no matching profile as a gh user on github.com, so a .ghx file
// works without any global config. Config validation keeps dirs, ssh_hosts and default
// pointing at real profiles, so only requested names reach the fallback.
func named(cfg config.Config, name string, source string) Match {
	profile, ok := cfg.Profiles[name]
	if !ok {
		profile = config.Profile{User: name}
	}
	return Match{ProfileName: name, Profile: profile, Source: source}
}

// fromOrigin runs git only when ssh_hosts or remote_user is configured, so other setups never pay for it.
func fromOrigin(cfg config.Config, current string, remoteURL RemoteURL) Match {
	if len(cfg.SSHHosts) == 0 && !cfg.RemoteUser {
		return Match{}
	}
	remote := remoteURL(current)
	if match := sshHost(cfg, remote); match.Found() {
		return match
	}
	if cfg.RemoteUser {
		return userInRemote(cfg, remote)
	}
	return Match{}
}

func sshHost(cfg config.Config, remote string) Match {
	host := remoteHost(remote)
	profileName, ok := cfg.SSHHosts[host]
	if !ok {
		return Match{}
	}
	return named(cfg, profileName, sshHostSource+host)
}

// userInRemote prefers the profile with that login and host, else uses the login directly.
// With profiles_only an unknown login is no match, so resolution falls through to dirs and default:
// a remote URL is not written as a profile selector.
func userInRemote(cfg config.Config, remote string) Match {
	user, host := remoteUser(remote)
	if user == "" {
		return Match{}
	}
	source := remoteUserSource + user + "@" + host
	for name, profile := range cfg.Profiles {
		if profile.User == user && profile.HostOrDefault() == host {
			return Match{ProfileName: name, Profile: profile, Source: source}
		}
	}
	if cfg.ProfilesOnly {
		return Match{}
	}
	profile := config.Profile{User: user}
	if host != config.DefaultHost {
		profile.Host = host
	}
	return Match{ProfileName: user, Profile: profile, Source: source}
}

// dirsMatch matches cwd itself first. Only when no dirs entry contains it, and cwd is in a linked
// worktree, is the worktree's main checkout matched instead.
func dirsMatch(cfg config.Config, current string) Match {
	match := deepestDir(cfg, current)
	if match.Found() || len(cfg.Dirs) == 0 || !cfg.FollowsWorktrees() {
		return match
	}
	mainCheckout := worktreeMainCheckout(current)
	if mainCheckout == "" {
		return Match{}
	}
	match = deepestDir(cfg, mainCheckout)
	if match.Found() {
		match.WorktreeMain = mainCheckout
	}
	return match
}

func deepestDir(cfg config.Config, current string) Match {
	best := Match{}
	bestLength := -1
	for dir, profileName := range cfg.Dirs {
		base := canonical(config.ExpandHome(dir))
		if !contains(base, current) || len(base) <= bestLength {
			continue
		}
		best = named(cfg, profileName, base)
		bestLength = len(base)
	}
	return best
}

func canonical(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		return resolved
	}
	return filepath.Clean(absolute)
}

func contains(base string, target string) bool {
	relative, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
