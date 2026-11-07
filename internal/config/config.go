package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	ManageGh       bool               `yaml:"manage_gh"`
	DisableUpdates bool               `yaml:"disable_updates"`
	SelfUpdate     *bool              `yaml:"self_update"`
	UpdateRepo     string             `yaml:"update_repo"`
	Default        string             `yaml:"default"`
	Profiles       map[string]Profile `yaml:"profiles"`
	Dirs           map[string]string  `yaml:"dirs"`
	SSHHosts       map[string]string  `yaml:"ssh_hosts"`
	RemoteUser     bool               `yaml:"remote_user"`
	ProfilesOnly   bool               `yaml:"profiles_only"`
	Name           string             `yaml:"name"`
	Email          string             `yaml:"email"`
	GitIdentity    string             `yaml:"git_identity"`
	// DirsFollowWorktrees is nil when the key is absent, which means on.
	DirsFollowWorktrees *bool `yaml:"dirs_follow_worktrees"`
}

// FollowsWorktrees reports whether a linked git worktree matches dirs through its main checkout.
func (cfg Config) FollowsWorktrees() bool {
	return cfg.DirsFollowWorktrees == nil || *cfg.DirsFollowWorktrees
}

// Load returns an empty Config when the file does not exist, so ghx acts as plain gh.
func Load(path string) (Config, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	return Parse(content)
}

func Parse(content []byte) (Config, error) {
	var parsed Config
	if err := yaml.Unmarshal(content, &parsed); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if err := parsed.validate(); err != nil {
		return Config{}, err
	}
	return parsed, nil
}

func (cfg Config) validate() error {
	if cfg.ProfilesOnly && len(cfg.Profiles) == 0 {
		return errors.New("profiles_only needs at least one profile")
	}
	for name, profile := range cfg.Profiles {
		if profile.User == "" {
			return fmt.Errorf("profile %q: user is required", name)
		}
		if !validIdentityMode(profile.GitIdentity) {
			return fmt.Errorf("profile %q: git_identity must be %q or %q, got %q", name, IdentityOnce, IdentityAlways, profile.GitIdentity)
		}
	}
	if !validIdentityMode(cfg.GitIdentity) {
		return fmt.Errorf("git_identity must be %q or %q, got %q", IdentityOnce, IdentityAlways, cfg.GitIdentity)
	}
	for dir, profileName := range cfg.Dirs {
		if _, ok := cfg.Profiles[profileName]; !ok {
			return fmt.Errorf("dir %q: unknown profile %q", dir, profileName)
		}
	}
	for host, profileName := range cfg.SSHHosts {
		if _, ok := cfg.Profiles[profileName]; !ok {
			return fmt.Errorf("ssh host %q: unknown profile %q", host, profileName)
		}
	}
	if _, ok := cfg.Profiles[cfg.Default]; cfg.Default != "" && !ok {
		return fmt.Errorf("default: unknown profile %q", cfg.Default)
	}
	if cfg.UpdateRepo != "" && !ValidUpdateRepo(cfg.UpdateRepo) {
		return fmt.Errorf("update_repo must be owner/repo, got %q", cfg.UpdateRepo)
	}
	return nil
}

var updateRepoPattern = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)

// ValidUpdateRepo reports whether repo is a GitHub owner/repo. The value is joined into an API
// URL, so "." and ".." are refused as the repo name.
func ValidUpdateRepo(repo string) bool {
	if !updateRepoPattern.MatchString(repo) {
		return false
	}
	_, name, _ := strings.Cut(repo, "/")
	return name != "." && name != ".."
}
