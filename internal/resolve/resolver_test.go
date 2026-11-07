package resolve

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/barnuri/ghx/internal/config"
)

func noRemote(string) string { return "" }

func mustProfile(t *testing.T, cfg config.Config, cwd string, override string, remoteURL RemoteURL) Match {
	t.Helper()
	match, err := Profile(cfg, cwd, override, remoteURL)
	if err != nil {
		t.Fatal(err)
	}
	return match
}

func newTree(t *testing.T) (string, config.Config) {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"work/api/src", "workshop", "personal/blog", "work/oss"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Config{
		Profiles: map[string]config.Profile{
			"work":     {User: "me-work"},
			"personal": {User: "me"},
		},
		Dirs: map[string]string{
			filepath.Join(root, "work"):     "work",
			filepath.Join(root, "personal"): "personal",
			filepath.Join(root, "work/oss"): "personal",
		},
	}
	return root, cfg
}

func TestProfile(t *testing.T) {
	root, cfg := newTree(t)
	cases := []struct {
		name string
		cwd  string
		want string
	}{
		{"configured dir itself", "work", "work"},
		{"nested subfolder inherits", "work/api/src", "work"},
		{"deepest configured dir wins", "work/oss", "personal"},
		{"sibling with shared prefix does not match", "workshop", ""},
		{"other tree", "personal/blog", "personal"},
		{"outside every dir", ".", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustProfile(t, cfg, filepath.Join(root, tc.cwd), "", noRemote)
			if got.ProfileName != tc.want {
				t.Fatalf("got %q, want %q", got.ProfileName, tc.want)
			}
		})
	}
}

func TestProfileOverrideWins(t *testing.T) {
	root, cfg := newTree(t)
	got := mustProfile(t, cfg, filepath.Join(root, "work"), "personal", noRemote)
	if got.ProfileName != "personal" || got.Profile.User != "me" || got.Source != overrideSource {
		t.Fatalf("unexpected match %+v", got)
	}
}

func TestProfileDefault(t *testing.T) {
	root, cfg := newTree(t)
	cfg.Default = "personal"
	if got := mustProfile(t, cfg, root, "", noRemote); got.ProfileName != "personal" || got.Source != defaultSource {
		t.Fatalf("unexpected match %+v", got)
	}
}

func TestRepoFileBeatsDirs(t *testing.T) {
	root, cfg := newTree(t)
	repoFile := filepath.Join(root, "work/api", RepoFileName)
	if err := os.WriteFile(repoFile, []byte("# repo profile\n\npersonal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := mustProfile(t, cfg, filepath.Join(root, "work/api/src"), "", noRemote)
	if got.ProfileName != "personal" || got.Source != canonical(repoFile) {
		t.Fatalf("unexpected match %+v", got)
	}
}

func TestRepoFileWithUnknownNameIsGhUser(t *testing.T) {
	root, cfg := newTree(t)
	if err := os.WriteFile(filepath.Join(root, "workshop", RepoFileName), []byte("octocat\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := mustProfile(t, cfg, filepath.Join(root, "workshop"), "", noRemote)
	if got.Profile.User != "octocat" || got.Profile.HostOrDefault() != config.DefaultHost {
		t.Fatalf("unexpected match %+v", got)
	}
}

func TestSymlinkedCwdResolvesToTarget(t *testing.T) {
	root, cfg := newTree(t)
	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Join(root, "work/api"), link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if got := mustProfile(t, cfg, link, "", noRemote); got.ProfileName != "work" {
		t.Fatalf("got %q, want work", got.ProfileName)
	}
}

func TestTildeDirExpands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(filepath.Join(home, "code/work/repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Profiles: map[string]config.Profile{"work": {User: "me-work"}},
		Dirs:     map[string]string{"~/code/work": "work"},
	}
	if got := mustProfile(t, cfg, filepath.Join(home, "code/work/repo"), "", noRemote); got.ProfileName != "work" {
		t.Fatalf("got %q, want work", got.ProfileName)
	}
}

func TestSSHHostBeatsDirs(t *testing.T) {
	root, cfg := newTree(t)
	cfg.SSHHosts = map[string]string{"gh-personal": "personal"}
	remote := func(string) string { return "git@gh-personal:org/repo.git" }
	got := mustProfile(t, cfg, filepath.Join(root, "work/api"), "", remote)
	if got.ProfileName != "personal" || got.Source != sshHostSource+"gh-personal" {
		t.Fatalf("unexpected match %+v", got)
	}
}

func TestSSHHostUnmappedFallsBackToDirs(t *testing.T) {
	root, cfg := newTree(t)
	cfg.SSHHosts = map[string]string{"gh-personal": "personal"}
	remote := func(string) string { return "git@github.com:org/repo.git" }
	if got := mustProfile(t, cfg, filepath.Join(root, "work/api"), "", remote); got.ProfileName != "work" {
		t.Fatalf("got %q, want work", got.ProfileName)
	}
}

func TestSSHHostSkipsGitWhenUnconfigured(t *testing.T) {
	root, cfg := newTree(t)
	remote := func(string) string {
		t.Fatal("remote lookup ran without ssh_hosts")
		return ""
	}
	mustProfile(t, cfg, filepath.Join(root, "work"), "", remote)
}

func TestRepoFileBeatsSSHHost(t *testing.T) {
	root, cfg := newTree(t)
	cfg.SSHHosts = map[string]string{"gh-personal": "personal"}
	if err := os.WriteFile(filepath.Join(root, "work", RepoFileName), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	remote := func(string) string { return "git@gh-personal:org/repo.git" }
	if got := mustProfile(t, cfg, filepath.Join(root, "work"), "", remote); got.ProfileName != "work" {
		t.Fatalf("got %q, want work", got.ProfileName)
	}
}

func TestRemoteUserMatchesProfileByLogin(t *testing.T) {
	root, cfg := newTree(t)
	cfg.RemoteUser = true
	remote := func(string) string { return "https://me@github.com/org/repo.git" }
	got := mustProfile(t, cfg, filepath.Join(root, "work/api"), "", remote)
	if got.ProfileName != "personal" || got.Source != remoteUserSource+"me@github.com" {
		t.Fatalf("unexpected match %+v", got)
	}
}

func TestRemoteUserWithoutProfileIsGhUser(t *testing.T) {
	root, cfg := newTree(t)
	cfg.RemoteUser = true
	remote := func(string) string { return "https://octocat@ghe.corp/org/repo.git" }
	got := mustProfile(t, cfg, filepath.Join(root, "work/api"), "", remote)
	if got.Profile.User != "octocat" || got.Profile.HostOrDefault() != "ghe.corp" {
		t.Fatalf("unexpected match %+v", got)
	}
}

func TestRemoteUserHostMismatchSkipsProfile(t *testing.T) {
	root, cfg := newTree(t)
	cfg.RemoteUser = true
	remote := func(string) string { return "https://me@ghe.corp/org/repo.git" }
	got := mustProfile(t, cfg, filepath.Join(root, "work/api"), "", remote)
	if got.ProfileName != "me" || got.Profile.HostOrDefault() != "ghe.corp" {
		t.Fatalf("unexpected match %+v", got)
	}
}

func TestRemoteWithoutUserFallsBackToDirs(t *testing.T) {
	root, cfg := newTree(t)
	cfg.RemoteUser = true
	remote := func(string) string { return "https://github.com/org/repo.git" }
	if got := mustProfile(t, cfg, filepath.Join(root, "work/api"), "", remote); got.ProfileName != "work" {
		t.Fatalf("got %q, want work", got.ProfileName)
	}
}

func TestRemoteUserIgnoredWhenDisabled(t *testing.T) {
	root, cfg := newTree(t)
	cfg.SSHHosts = map[string]string{"gh-personal": "personal"}
	remote := func(string) string { return "https://me@github.com/org/repo.git" }
	if got := mustProfile(t, cfg, filepath.Join(root, "work/api"), "", remote); got.ProfileName != "work" {
		t.Fatalf("got %q, want work", got.ProfileName)
	}
}

func TestRemoteLookupRunsOnceWithBothKeys(t *testing.T) {
	root, cfg := newTree(t)
	cfg.SSHHosts = map[string]string{"gh-personal": "personal"}
	cfg.RemoteUser = true
	calls := 0
	remote := func(string) string {
		calls++
		return "https://me@github.com/org/repo.git"
	}
	mustProfile(t, cfg, filepath.Join(root, "work/api"), "", remote)
	if calls != 1 {
		t.Fatalf("remote lookup ran %d times, want 1", calls)
	}
}

func TestProfilesOnly(t *testing.T) {
	cases := []struct {
		name         string
		profilesOnly bool
		override     string
		repoFile     string
		remote       string
		cwd          string
		wantProfile  string
		wantUser     string
		wantSource   string // "file" stands for the .ghx path
	}{
		{"override names a profile", true, "personal", "", "", "work", "personal", "me", overrideSource},
		{"override names no profile", true, "stranger", "", "", "work", "", "", overrideSource},
		{"override names no profile, key off", false, "stranger", "", "", "work", "stranger", "stranger", overrideSource},
		{".ghx names a profile", true, "", "personal", "", "work/api", "personal", "me", "file"},
		{".ghx names no profile", true, "", "stranger", "", "work/api", "", "", "file"},
		{".ghx names no profile, key off", false, "", "stranger", "", "work/api", "stranger", "stranger", "file"},
		{"remote user matches a profile", true, "", "", "https://me@github.com/org/repo.git", "work/api", "personal", "me", remoteUserSource + "me@github.com"},
		{"remote user unknown falls through to dirs", true, "", "", "https://stranger@github.com/org/repo.git", "work/api", "work", "me-work", ""},
		{"remote user unknown, key off", false, "", "", "https://stranger@github.com/org/repo.git", "work/api", "stranger", "stranger", remoteUserSource + "stranger@github.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, cfg := newTree(t)
			cfg.ProfilesOnly = tc.profilesOnly
			cfg.RemoteUser = true
			remote := func(string) string { return tc.remote }
			repoFile := filepath.Join(root, "work/api", RepoFileName)
			if tc.repoFile != "" {
				if err := os.WriteFile(repoFile, []byte(tc.repoFile+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			wantSource := tc.wantSource
			if wantSource == "file" {
				wantSource = canonical(repoFile)
			}
			got, err := Profile(cfg, filepath.Join(root, tc.cwd), tc.override, remote)
			if tc.wantProfile == "" {
				var unknown *UnknownProfileError
				if !errors.As(err, &unknown) {
					t.Fatalf("got %+v, %v; want an UnknownProfileError", got, err)
				}
				if unknown.Source != wantSource || got.Found() {
					t.Fatalf("got %+v, source %q; want no match, source %q", got, unknown.Source, wantSource)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.ProfileName != tc.wantProfile || got.Profile.User != tc.wantUser {
				t.Fatalf("got %+v, want profile %q as %q", got, tc.wantProfile, tc.wantUser)
			}
			if wantSource != "" && got.Source != wantSource {
				t.Fatalf("got source %q, want %q", got.Source, wantSource)
			}
		})
	}
}

func TestUnknownProfileErrorNamesSource(t *testing.T) {
	err := &UnknownProfileError{Name: "stranger", Source: "/repo/.ghx"}
	if got := err.Error(); got != `/repo/.ghx names "stranger", which is not a profile (profiles_only: true)` {
		t.Fatalf("got %s", got)
	}
}
