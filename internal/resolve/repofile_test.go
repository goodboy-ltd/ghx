package resolve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/barnuri/ghx/internal/config"
)

// scopeTree lays out base/{home/proj/sub, home/wt, repo/sub, outside/sub} with HOME at base/home.
func scopeTree(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	for _, dir := range []string{"home/proj/sub", "home/wt", "repo/sub", "outside/sub"} {
		if err := os.MkdirAll(filepath.Join(base, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	home := filepath.Join(base, "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return base
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRepoFileScope(t *testing.T) {
	cases := []struct {
		name     string
		lenient  bool              // ghx_file_strict: false
		git      map[string]string // folder -> "dir" or "file" (a worktree's .git)
		files    map[string]string // folder -> .ghx content
		cwd      string
		wantName string
		wantIn   string // folder of the .ghx that wins
	}{
		{"above the repo root is not read", false, map[string]string{"repo": "dir"}, map[string]string{".": "personal"}, "repo/sub", "", ""},
		{"at the repo root is read", false, map[string]string{"repo": "dir"}, map[string]string{"repo": "work"}, "repo/sub", "work", "repo"},
		{"below the repo root is read", false, map[string]string{"repo": "dir"}, map[string]string{"repo/sub": "work", "repo": "personal"}, "repo/sub", "work", "repo/sub"},
		{"a .git file marks a worktree root", false, map[string]string{"home/wt": "file"}, map[string]string{"home": "personal"}, "home/wt", "", ""},
		{"a worktree root's own .ghx is read", false, map[string]string{"home/wt": "file"}, map[string]string{"home/wt": "work"}, "home/wt", "work", "home/wt"},
		{"outside a repo, home is read", false, nil, map[string]string{"home": "work"}, "home/proj/sub", "work", "home"},
		{"outside a repo, above home is not read", false, nil, map[string]string{".": "personal"}, "home/proj/sub", "", ""},
		{"outside home and any repo, nothing is read", false, nil, map[string]string{"outside/sub": "work"}, "outside/sub", "", ""},
		{"ghx_file_strict false walks past the repo root", true, map[string]string{"repo": "dir"}, map[string]string{".": "personal"}, "repo/sub", "personal", "."},
		{"ghx_file_strict false reads outside home", true, nil, map[string]string{"outside/sub": "work"}, "outside/sub", "work", "outside/sub"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := scopeTree(t)
			for dir, kind := range tc.git {
				entry := filepath.Join(base, dir, gitEntry)
				if kind == "file" {
					writeFile(t, entry, "gitdir: /elsewhere/.git/worktrees/wt\n")
				} else if err := os.Mkdir(entry, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for dir, content := range tc.files {
				writeFile(t, filepath.Join(base, dir, RepoFileName), content+"\n")
			}
			strict := !tc.lenient
			cfg := config.Config{GhxFileStrict: &strict}
			got := mustProfile(t, cfg, filepath.Join(base, tc.cwd), "", noRemote)
			if got.ProfileName != tc.wantName {
				t.Fatalf("got %+v, want profile %q", got, tc.wantName)
			}
			if want := canonical(filepath.Join(base, tc.wantIn, RepoFileName)); tc.wantName != "" && got.Source != want {
				t.Fatalf("got source %q, want %q", got.Source, want)
			}
		})
	}
}

func TestStrictRepoFileDefaultsOn(t *testing.T) {
	base := scopeTree(t)
	writeFile(t, filepath.Join(base, RepoFileName), "personal\n")
	if err := os.Mkdir(filepath.Join(base, "repo", gitEntry), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := mustProfile(t, config.Config{}, filepath.Join(base, "repo/sub"), "", noRemote); got.Found() {
		t.Fatalf("a .ghx above the repo root was read with the default config: %+v", got)
	}
}
