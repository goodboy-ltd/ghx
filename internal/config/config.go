package config

import (
	"errors"
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	ManageGh       bool               `yaml:"manage_gh"`
	DisableUpdates bool               `yaml:"disable_updates"`
	Default        string             `yaml:"default"`
	Profiles       map[string]Profile `yaml:"profiles"`
	Dirs           map[string]string  `yaml:"dirs"`
	SSHHosts       map[string]string  `yaml:"ssh_hosts"`
	RemoteUser     bool               `yaml:"remote_user"`
	ProfilesOnly   bool               `yaml:"profiles_only"`
	GhxFileStrict  *bool              `yaml:"ghx_file_strict"`
	Name           string             `yaml:"name"`
	Email          string             `yaml:"email"`
	GitIdentity    string             `yaml:"git_identity"`
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

// StrictRepoFile reports ghx_file_strict, which defaults to true: the .ghx walk stops at the
// repository's top folder (or home), and on Unix skips files someone else could have written.
func (cfg Config) StrictRepoFile() bool {
	return cfg.GhxFileStrict == nil || *cfg.GhxFileStrict
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
	return nil
}
