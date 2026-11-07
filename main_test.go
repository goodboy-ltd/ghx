package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/barnuri/ghx/internal/config"
	"github.com/barnuri/ghx/internal/resolve"
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

func TestPrintWhichReportsTokenOverride(t *testing.T) {
	matched := resolve.Match{ProfileName: "work", Profile: config.Profile{User: "octocat"}, Source: "/tmp/.ghx"}
	cases := map[string]struct {
		match resolve.Match
		env   []string
		want  string
	}{
		"GH_TOKEN set":            {matched, []string{"GH_TOKEN=x"}, "token:   overridden by GH_TOKEN"},
		"GH_ENTERPRISE_TOKEN set": {matched, []string{"GH_ENTERPRISE_TOKEN=x"}, "token:   overridden by GH_ENTERPRISE_TOKEN"},
		"no token":                {matched, []string{"PATH=/bin"}, ""},
		"no profile":              {resolve.Match{}, []string{"GH_TOKEN=x"}, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			printWhich(&out, tc.match, nil, "/usr/bin/gh", tc.env, update.Policy{})
			got := out.String()
			if tc.want == "" {
				if strings.Contains(got, "token:") {
					t.Fatalf("unexpected token line in %q", got)
				}
				return
			}
			if !strings.Contains(got, "\n"+tc.want) {
				t.Fatalf("output %q lacks %q", got, tc.want)
			}
		})
	}
}

func TestPrintWhichShowsRefusedProfileSource(t *testing.T) {
	refused := &resolve.UnknownProfileError{Name: "stranger", Source: "/repo/.ghx"}
	var out bytes.Buffer
	printWhich(&out, resolve.Match{}, refused, "/usr/bin/gh", []string{"GH_TOKEN=x"}, update.Policy{})
	got := out.String()
	for _, want := range []string{"profile: stranger (refused: not a profile, profiles_only: true)\n", "source:  /repo/.ghx\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output %q lacks %q", got, want)
		}
	}
	if strings.Contains(got, "token:") {
		t.Fatalf("unexpected token line in %q", got)
	}
}

func TestPrintWhichShowsUpdatePolicy(t *testing.T) {
	matched := resolve.Match{ProfileName: "work", Profile: config.Profile{User: "octocat"}, Source: "/tmp/.ghx"}
	refused := &resolve.UnknownProfileError{Name: "stranger", Source: "/repo/.ghx"}
	policy := update.Policy{Repo: "goodboy-ltd/ghx", Reason: "build default"}
	cases := map[string]struct {
		match      resolve.Match
		resolveErr error
	}{
		"profile":    {matched, nil},
		"no profile": {resolve.Match{}, nil},
		"refused":    {resolve.Match{}, refused},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			printWhich(&out, tc.match, tc.resolveErr, "/usr/bin/gh", nil, policy)
			if got := out.String(); !strings.Contains(got, "\nupdates: off (build default)\n") {
				t.Fatalf("output %q lacks the updates line", got)
			}
		})
	}
}
