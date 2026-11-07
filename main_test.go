package main

import (
	"strings"
	"testing"

	"github.com/barnuri/ghx/internal/config"
	"github.com/barnuri/ghx/internal/update"
)

func TestUpdatePlan(t *testing.T) {
	upstream := buildVars{updateRepo: "barnuri/ghx", selfUpdate: "on"}
	fork := buildVars{updateRepo: "goodboy-ltd/ghx", selfUpdate: "off"}
	const (
		plainPath = "/Users/me/.local/bin/ghx"
		brewPath  = "/opt/homebrew/Cellar/ghx/0.2.0/bin/ghx"
		asdfPath  = "/Users/me/.asdf/installs/ghx/0.2.0/bin/ghx"
	)
	on, off := true, false
	cases := []struct {
		name       string
		cfg        config.Config
		build      buildVars
		executable string
		want       update.Policy
		background bool
	}{
		{"upstream defaults", config.Config{}, upstream, plainPath,
			update.Policy{Repo: "barnuri/ghx", Enabled: true, Reason: "build default"}, true},
		{"upstream, self_update false", config.Config{SelfUpdate: &off}, upstream, plainPath,
			update.Policy{Repo: "barnuri/ghx", Reason: "self_update: false"}, false},
		{"upstream, self_update true", config.Config{SelfUpdate: &on}, upstream, plainPath,
			update.Policy{Repo: "barnuri/ghx", Enabled: true, Reason: "self_update: true"}, true},
		{"upstream, update_repo", config.Config{UpdateRepo: "goodboy-ltd/ghx"}, upstream, plainPath,
			update.Policy{Repo: "goodboy-ltd/ghx", Enabled: true, Reason: "build default"}, true},
		{"upstream, disable_updates", config.Config{DisableUpdates: true}, upstream, plainPath,
			update.Policy{Repo: "barnuri/ghx", Enabled: true, Reason: "build default"}, false},
		{"fork defaults", config.Config{}, fork, plainPath,
			update.Policy{Repo: "goodboy-ltd/ghx", Reason: "build default"}, false},
		{"fork, self_update true", config.Config{SelfUpdate: &on}, fork, plainPath,
			update.Policy{Repo: "goodboy-ltd/ghx", Enabled: true, Reason: "self_update: true"}, true},
		{"fork, self_update false", config.Config{SelfUpdate: &off}, fork, plainPath,
			update.Policy{Repo: "goodboy-ltd/ghx", Reason: "self_update: false"}, false},
		{"fork, update_repo and self_update true", config.Config{SelfUpdate: &on, UpdateRepo: "barnuri/ghx"}, fork, plainPath,
			update.Policy{Repo: "barnuri/ghx", Enabled: true, Reason: "self_update: true"}, true},
		{"fork, manage_gh still checks gh", config.Config{ManageGh: true}, fork, plainPath,
			update.Policy{Repo: "goodboy-ltd/ghx", Reason: "build default"}, true},
		{"fork, manage_gh and disable_updates", config.Config{ManageGh: true, DisableUpdates: true}, fork, plainPath,
			update.Policy{Repo: "goodboy-ltd/ghx", Reason: "build default"}, false},
		{"homebrew Cellar path", config.Config{}, upstream, brewPath,
			update.Policy{Repo: "barnuri/ghx", Enabled: true, Reason: "build default", ManagedBy: "homebrew"}, false},
		{"homebrew Cellar path, self_update true", config.Config{SelfUpdate: &on}, fork, brewPath,
			update.Policy{Repo: "goodboy-ltd/ghx", Enabled: true, Reason: "self_update: true", ManagedBy: "homebrew"}, false},
		{"asdf installs path", config.Config{}, upstream, asdfPath,
			update.Policy{Repo: "barnuri/ghx", Enabled: true, Reason: "build default", ManagedBy: "asdf"}, false},
		{"asdf path, manage_gh", config.Config{ManageGh: true}, upstream, asdfPath,
			update.Policy{Repo: "barnuri/ghx", Enabled: true, Reason: "build default", ManagedBy: "asdf"}, true},
		{"build packageManager homebrew", config.Config{}, buildVars{"barnuri/ghx", "on", "homebrew"}, plainPath,
			update.Policy{Repo: "barnuri/ghx", Enabled: true, Reason: "build default", ManagedBy: "homebrew"}, false},
		{"build packageManager wins over path", config.Config{}, buildVars{"barnuri/ghx", "on", "asdf"}, brewPath,
			update.Policy{Repo: "barnuri/ghx", Enabled: true, Reason: "build default", ManagedBy: "asdf"}, false},
		{"unknown executable", config.Config{}, upstream, "",
			update.Policy{Repo: "barnuri/ghx", Enabled: true, Reason: "build default"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := updatePlan(tc.cfg, tc.build, tc.executable)
			if err != nil {
				t.Fatal(err)
			}
			if got.Self != tc.want || got.Background != tc.background {
				t.Fatalf("got %+v, want %+v background=%t", got, tc.want, tc.background)
			}
		})
	}
}

func TestUpdatePlanRejectsInvalidBuildVars(t *testing.T) {
	cases := map[string]buildVars{
		"selfUpdate":       {updateRepo: "barnuri/ghx", selfUpdate: "true"},
		"empty selfUpdate": {updateRepo: "barnuri/ghx"},
		"packageManager":   {updateRepo: "barnuri/ghx", selfUpdate: "on", packageManager: "apt"},
		"updateRepo":       {updateRepo: "barnuri", selfUpdate: "on"},
		"updateRepo dots":  {updateRepo: "barnuri/..", selfUpdate: "on"},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := updatePlan(config.Config{}, build, "")
			if err == nil || !strings.Contains(err.Error(), "invalid build setting") {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestDefaultBuildVarsAreUpstreams(t *testing.T) {
	if updateRepo != "barnuri/ghx" || selfUpdate != "on" || packageManager != "" {
		t.Fatalf("defaults changed: %q %q %q", updateRepo, selfUpdate, packageManager)
	}
}
