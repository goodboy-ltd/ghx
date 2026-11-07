package update

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectPackageManager(t *testing.T) {
	cases := map[string]string{
		"/opt/homebrew/Cellar/ghx/0.2.0/bin/ghx":                PackageManagerHomebrew,
		"/usr/local/Cellar/ghx/0.2.0/bin/ghx":                   PackageManagerHomebrew,
		"/home/linuxbrew/.linuxbrew/Cellar/ghx/0.2.0_1/bin/ghx": PackageManagerHomebrew,
		"/Users/me/.asdf/installs/ghx/0.2.0/bin/ghx":            PackageManagerAsdf,
		"/home/me/.local/share/asdf/installs/ghx/0.2.0/bin/ghx": PackageManagerAsdf,
		"/Users/me/.local/bin/ghx":                              "",
		"/opt/homebrew/bin/ghx":                                 "",
		"/opt/homebrew/Cellar/other/1.0/bin/ghx":                "",
		"/opt/homebrew/Cellar/ghx/0.2.0/libexec/ghx":            "",
		"/Users/me/.asdf/installs/ghx/0.2.0/ghx":                "",
		"ghx":                                                   "",
		"":                                                      "",
	}
	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			if got := DetectPackageManager(path); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestPolicyDescribe(t *testing.T) {
	cases := map[string]Policy{
		"off (build default)":      {Repo: "o/r", Reason: "build default"},
		"off (self_update: false)": {Repo: "o/r", Enabled: false, Reason: "self_update: false"},
		"on from goodboy-ltd/ghx":  {Repo: "goodboy-ltd/ghx", Enabled: true},
		"managed by homebrew":      {Repo: "o/r", Enabled: true, ManagedBy: PackageManagerHomebrew},
		"managed by asdf":          {Repo: "o/r", Reason: "build default", ManagedBy: PackageManagerAsdf},
	}
	for want, policy := range cases {
		if got := policy.Describe(); got != want {
			t.Fatalf("%+v: got %q, want %q", policy, got, want)
		}
	}
}

func TestUpdaterRunReportsDisabledSelfUpdate(t *testing.T) {
	var out bytes.Buffer
	updater := Updater{Self: SelfUpdater{CurrentVersion: "1.0.0", Policy: Policy{Repo: "o/r", Reason: "build default"}}}
	if err := updater.Run(&out); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "ghx self-update is off (build default)" {
		t.Fatalf("got %q", got)
	}
}

func TestUpdaterRunFailsForManagedInstall(t *testing.T) {
	var out bytes.Buffer
	updater := Updater{Self: SelfUpdater{CurrentVersion: "1.0.0", Policy: Policy{Repo: "o/r", Enabled: true, ManagedBy: PackageManagerAsdf}}}
	err := updater.Run(&out)
	if err == nil || err.Error() != "update failed: ghx: ghx is installed by asdf; run: asdf install ghx latest" {
		t.Fatalf("got %v", err)
	}
}

func TestRunQuietlySkipsInactiveSelfUpdate(t *testing.T) {
	stateDir := t.TempDir()
	updater := Updater{StateDir: stateDir, Self: SelfUpdater{CurrentVersion: "1.0.0", Policy: Policy{Repo: "o/r", Enabled: true, ManagedBy: PackageManagerHomebrew}}}
	updater.RunQuietly()
	content, err := os.ReadFile(filepath.Join(stateDir, logName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "ghx") {
		t.Fatalf("expected no ghx step in the background log, got %q", content)
	}
}
